package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/client"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var AppVersion = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := newRootCommand().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var jsonOutput bool
	var serverURL string
	root := &cobra.Command{Use: "owctl", Short: "Manage Overwatcher from the terminal", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Print raw API JSON")
	root.PersistentFlags().StringVar(&serverURL, "url", "", "Coordinator URL (overrides environment and config)")
	resolve := func() (config, error) {
		cfg, err := loadConfig()
		if root.PersistentFlags().Changed("url") {
			cfg.URL = serverURL
		}
		return cfg, err
	}
	api := func() (*client.Client, error) {
		cfg, err := resolve()
		if err != nil {
			return nil, err
		}
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("no API key: run owctl login or set OVERWATCHER_API_KEY")
		}
		return client.New(cfg.URL, cfg.APIKey)
	}
	login := &cobra.Command{Use: "login", Short: "Save an API key (input is hidden)", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := resolve()
		if err != nil {
			return err
		}
		if _, err := client.New(cfg.URL, ""); err != nil {
			return err
		}
		input, ok := cmd.InOrStdin().(*os.File)
		if !ok || !term.IsTerminal(int(input.Fd())) {
			return fmt.Errorf("login requires a terminal; use OVERWATCHER_API_KEY for non-interactive access")
		}
		key, err := readPassword(input, cmd.ErrOrStderr())
		fmt.Fprintln(cmd.ErrOrStderr())
		if err != nil {
			return fmt.Errorf("read API key: %w", err)
		}
		cfg.APIKey = strings.TrimSpace(string(key))
		if cfg.APIKey == "" {
			return fmt.Errorf("API key must not be empty")
		}
		if err := saveConfig(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "API key saved.")
		return err
	}}
	project := &cobra.Command{Use: "project", Short: "Manage projects"}
	project.AddCommand(&cobra.Command{Use: "list", Short: "List your projects", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := api()
		if err != nil {
			return err
		}
		response, err := c.ListProjects(cmd.Context())
		if err != nil {
			return err
		}
		if jsonOutput {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(response.Raw))
			return err
		}
		return printProjects(cmd, response.Data.Projects)
	}})
	project.AddCommand(&cobra.Command{Use: "get <project>", Short: "Get a project by name or ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := api()
		if err != nil {
			return err
		}
		id, err := resolveProject(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		response, err := c.GetProject(cmd.Context(), id)
		if err != nil {
			return err
		}
		if jsonOutput {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(response.Raw))
			return err
		}
		return printProjects(cmd, []dto.ProjectResponse{response.Data})
	}})
	version := &cobra.Command{Use: "version", Short: "Show client and server versions", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := api()
		if err != nil {
			return err
		}
		response, err := c.Version(cmd.Context())
		if err != nil {
			return err
		}
		if jsonOutput {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(response.Raw))
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Client: %s\nServer: %s\n", AppVersion, response.Data.Version)
		return err
	}}
	root.AddCommand(login, project, version)
	addManagementCommands(root, project, api, &jsonOutput)
	return root
}

func printProjects(cmd *cobra.Command, projects []dto.ProjectResponse) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "ID\tNAME\tENVIRONMENT\tENABLED"); err != nil {
		return err
	}
	for _, project := range projects {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%t\n", project.ID, tableText(project.Name), tableText(project.Environment), project.Enabled); err != nil {
			return err
		}
	}
	return w.Flush()
}

func tableText(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, value)
}
