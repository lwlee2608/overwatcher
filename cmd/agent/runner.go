package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
)

// Runner executes a deploy intent by shelling out to `docker compose`.
type Runner struct {
	stacks map[string]string
}

func NewRunner(stacks map[string]string) *Runner {
	return &Runner{stacks: stacks}
}

// Run resolves the compose file from the stack map, then runs `pull` followed
// by `up -d`. Returns nil on success or an error describing which command
// failed and its combined output.
func (r *Runner) Run(ctx context.Context, intent *dto.DeployIntentResponse) error {
	composeFile, ok := r.stacks[intent.Stack]
	if !ok {
		return fmt.Errorf("no compose file configured for stack %q", intent.Stack)
	}

	if err := r.runCompose(ctx, composeFile, "pull"); err != nil {
		return fmt.Errorf("docker compose pull: %w", err)
	}

	upArgs := []string{"compose", "-f", composeFile, "up", "-d"}
	upArgs = append(upArgs, intent.Services...)
	if err := r.runDocker(ctx, upArgs...); err != nil {
		return fmt.Errorf("docker compose up: %w", err)
	}
	return nil
}

func (r *Runner) runCompose(ctx context.Context, composeFile string, sub string, extra ...string) error {
	args := append([]string{"compose", "-f", composeFile, sub}, extra...)
	return r.runDocker(ctx, args...)
}

func (r *Runner) runDocker(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	slog.Info("docker exec", "args", args, "output_bytes", len(out))
	if err != nil {
		slog.Error("docker exec failed", "args", args, "output", string(out), "error", err)
		return fmt.Errorf("%w: %s", err, string(out))
	}
	slog.Debug("docker exec output", "args", args, "output", string(out))
	return nil
}
