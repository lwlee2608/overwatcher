package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/spf13/cobra"
)

var sshTargetPattern = regexp.MustCompile(`^([a-zA-Z0-9_][a-zA-Z0-9_.-]*@)?[a-zA-Z0-9_:\[][a-zA-Z0-9_.:\]-]*$`)

const agentPreflight = `set -eu
command -v systemctl >/dev/null
if systemctl is-active --quiet overwatcher-agent; then
 exit 20
else
 status=$?
 case "$status" in 3|4) ;; *) exit "$status" ;; esac
fi
docker compose version >/dev/null 2>&1 || exit 21
`

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

func runSSH(ctx context.Context, target, script string) error {
	command := exec.CommandContext(ctx, "ssh", "-T", "-o", "BatchMode=yes", "--", target, "sh -s")
	command.Stdin = strings.NewReader(script)
	// Remote installer diagnostics may contain the token. Never forward them.
	command.WaitDelay = time.Second
	return command.Run()
}

func readConfirmation(ctx context.Context, input io.Reader) (string, error) {
	type result struct {
		answer string
		err    error
	}
	ready := make(chan result, 1)
	go func() {
		answer, err := bufio.NewReader(input).ReadString('\n')
		ready <- result{answer, err}
	}()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case response := <-ready:
		return response.answer, response.err
	}
}

func newAgentInstallCommand(api clientFactory, jsonOutput *bool) *cobra.Command {
	var target, project, name string
	var yes bool
	var timeout time.Duration
	cmd := &cobra.Command{Use: "install --ssh <target>", Short: "Install an agent over SSH (requires passwordless sudo)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (resultErr error) {
			if !sshTargetPattern.MatchString(target) {
				return fmt.Errorf("invalid SSH target: use [user@]host (SSH config aliases are supported)")
			}
			if timeout <= 0 {
				return fmt.Errorf("timeout must be positive")
			}
			if name == "" {
				name = target[strings.LastIndex(target, "@")+1:]
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "SSH target: %s\n", target)
			if !yes {
				fmt.Fprint(cmd.ErrOrStderr(), "Install Overwatcher agent? [y/N]: ")
				answer, err := readConfirmation(cmd.Context(), cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("read confirmation: %w", err)
				}
				if answer = strings.ToLower(strings.TrimSpace(answer)); answer != "y" && answer != "yes" {
					return fmt.Errorf("installation cancelled")
				}
			}
			c, err := api()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			var projectID string
			if project != "" {
				projectID, err = resolveProject(ctx, c, project)
				if err != nil {
					return err
				}
				project, err := c.GetProject(ctx, projectID)
				if err != nil {
					return err
				}
				if project.Data.Role != "owner" {
					return fmt.Errorf("install requires project ownership to bind an agent")
				}
			}
			if err := runSSH(ctx, target, agentPreflight); err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					switch exit.ExitCode() {
					case 20:
						return fmt.Errorf("preflight: overwatcher-agent is already active")
					case 21:
						return fmt.Errorf("preflight: docker compose is required")
					}
				}
				return fmt.Errorf("SSH preflight failed (check SSH access and systemctl): %w", err)
			}
			created, err := c.CreateAgent(ctx, dto.CreateAgentRequest{Name: name})
			if err != nil {
				return err
			}
			id := created.Data.AgentID
			complete := false
			bindAttempted := false
			defer func() {
				if complete {
					return
				}
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cleanupCancel()
				if bindAttempted {
					// A failed response can follow a committed binding. Unbind before
					// deleting so that ambiguous failures still revoke this agent.
					if _, err := c.BindAgent(cleanupCtx, id, dto.BindAgentProjectRequest{}); err != nil {
						resultErr = fmt.Errorf("%w; cleanup unbind failed for agent %s: %v", resultErr, id, err)
					}
				}
				if err := c.DeleteAgent(cleanupCtx, id); err != nil {
					resultErr = fmt.Errorf("%w; cleanup failed for agent %s: %v", resultErr, id, err)
				} else {
					resultErr = fmt.Errorf("%w; created agent deleted; remote files/service may remain on %s", resultErr, target)
				}
			}()
			// Send secrets only over SSH stdin, never in local or remote command arguments.
			script := agentPreflight + `tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
curl -fsSL -o "$tmp" -- ` + shellQuote(c.ServerURL()+"/install.sh") + `
{ printf '%s\n' ` + shellQuote("export AGENT_TOKEN="+shellQuote(created.Data.Token)) + `; cat "$tmp"; } | sudo -n bash
`
			if err := runSSH(ctx, target, script); err != nil {
				return fmt.Errorf("SSH install failed: %w", err)
			}
			for {
				agent, err := c.GetAgent(ctx, id)
				if err != nil {
					return fmt.Errorf("wait for agent connection: %w", err)
				}
				if agent.Data.LastSeen != nil {
					if projectID != "" {
						bindAttempted = true
						agent, err = c.BindAgent(ctx, id, dto.BindAgentProjectRequest{ProjectID: projectID})
						if err != nil {
							return fmt.Errorf("bind agent: %w", err)
						}
					}
					complete = true
					if *jsonOutput {
						return printRaw(cmd, agent.Raw)
					}
					return printAgents(cmd, []dto.AgentStatusResponse{agent.Data})
				}
				select {
				case <-ctx.Done():
					return fmt.Errorf("wait for agent connection: %w", ctx.Err())
				case <-time.After(250 * time.Millisecond):
				}
			}
		}}
	cmd.Flags().StringVar(&target, "ssh", "", "SSH target ([user@]host)")
	cmd.Flags().StringVar(&project, "project", "", "Project name or ID to bind after connection")
	cmd.Flags().StringVar(&name, "name", "", "Agent name (defaults to SSH host)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation")
	cmd.Flags().DurationVar(&timeout, "timeout", 60*time.Second, "Maximum time for preflight, install and connection")
	_ = cmd.MarkFlagRequired("ssh")
	return cmd
}
