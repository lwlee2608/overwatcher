package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/google/uuid"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/client"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type clientFactory func() (*client.Client, error)

func resolveProject(ctx context.Context, c *client.Client, reference string) (string, error) {
	if _, err := uuid.Parse(reference); err == nil {
		return reference, nil
	}
	response, err := c.ListProjects(ctx)
	if err != nil {
		return "", err
	}
	var id string
	for _, project := range response.Data.Projects {
		if project.Name != reference {
			continue
		}
		if id != "" {
			return "", fmt.Errorf("project name %q is ambiguous; use a project ID", reference)
		}
		id = project.ID
	}
	if id == "" {
		return "", fmt.Errorf("project not found: %s", reference)
	}
	return id, nil
}

func addManagementCommands(root, project *cobra.Command, api clientFactory, jsonOutput *bool) {
	var createInput dto.CreateProjectRequest
	var enabled bool
	create := &cobra.Command{Use: "create <name> --compose-file <path>", Short: "Create a project", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := api()
		if err != nil {
			return err
		}
		createInput.Name = args[0]
		if cmd.Flags().Changed("enabled") {
			createInput.Enabled = &enabled
		}
		response, err := c.CreateProject(cmd.Context(), createInput)
		if err != nil {
			return err
		}
		if *jsonOutput {
			return printRaw(cmd, response.Raw)
		}
		return printProjects(cmd, []dto.ProjectResponse{response.Data})
	}}
	create.Flags().StringVar(&createInput.ComposeFile, "compose-file", "", "Compose file path on the agent")
	create.Flags().StringVar(&createInput.Description, "description", "", "Project description")
	create.Flags().StringVar(&createInput.Environment, "environment", "", "Project environment")
	create.Flags().BoolVar(&enabled, "enabled", true, "Enable deployments")
	_ = create.MarkFlagRequired("compose-file")
	project.AddCommand(create, &cobra.Command{Use: "delete <project>", Short: "Delete a project by name or ID", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := api()
		if err != nil {
			return err
		}
		id, err := resolveProject(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		if err := c.DeleteProject(cmd.Context(), id); err != nil {
			return err
		}
		// DELETE returns 204: there is no API JSON to print.
		if *jsonOutput {
			return nil
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Project deleted.")
		return err
	}})
	var filename string
	service := &cobra.Command{Use: "service", Short: "Manage project services"}
	set := &cobra.Command{Use: "set <project> -f <file>", Short: "Replace all services from YAML or JSON (- for stdin)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		input := cmd.InOrStdin()
		if filename != "-" {
			file, err := os.Open(filename)
			if err != nil {
				return err
			}
			defer file.Close()
			input = file
		}
		payload, err := readServices(input)
		if err != nil {
			return err
		}
		c, err := api()
		if err != nil {
			return err
		}
		id, err := resolveProject(cmd.Context(), c, args[0])
		if err != nil {
			return err
		}
		response, err := c.ReplaceServices(cmd.Context(), id, payload)
		if err != nil {
			return err
		}
		if *jsonOutput {
			return printRaw(cmd, response.Raw)
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tREPO\tIMAGE\tTAG")
		for _, s := range response.Data.Services {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.ID, tableText(s.Name), tableText(s.Repo), tableText(s.Image), tableText(s.Tag))
		}
		return w.Flush()
	}}
	set.Flags().StringVarP(&filename, "file", "f", "", "Services request file (- for stdin)")
	_ = set.MarkFlagRequired("file")
	service.AddCommand(set)
	agent := &cobra.Command{Use: "agent", Short: "Manage agents"}
	agent.AddCommand(&cobra.Command{Use: "list", Short: "List your agents", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := api()
		if err != nil {
			return err
		}
		response, err := c.ListAgents(cmd.Context())
		if err != nil {
			return err
		}
		if *jsonOutput {
			return printRaw(cmd, response.Raw)
		}
		return printAgents(cmd, response.Data.Agents)
	}}, &cobra.Command{Use: "bind <agent-id> <project>", Short: "Bind an agent to a project by name or ID", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		c, err := api()
		if err != nil {
			return err
		}
		id, err := resolveProject(cmd.Context(), c, args[1])
		if err != nil {
			return err
		}
		response, err := c.BindAgent(cmd.Context(), args[0], dto.BindAgentProjectRequest{ProjectID: id})
		if err != nil {
			return err
		}
		if *jsonOutput {
			return printRaw(cmd, response.Raw)
		}
		return printAgents(cmd, []dto.AgentStatusResponse{response.Data})
	}})
	root.AddCommand(service, agent)
}

func readServices(input io.Reader) (dto.ReplaceComposeServicesRequest, error) {
	var result dto.ReplaceComposeServicesRequest
	decoder := yaml.NewDecoder(input)
	var value any
	if err := decoder.Decode(&value); err != nil {
		return result, fmt.Errorf("read services: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return result, fmt.Errorf("services input must contain exactly one document")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return result, fmt.Errorf("read services: %w", err)
	}
	// Decode through JSON so the shared DTO's JSON field names also govern YAML.
	jsonDecoder := json.NewDecoder(strings.NewReader(string(data)))
	jsonDecoder.DisallowUnknownFields()
	if err := jsonDecoder.Decode(&result); err != nil {
		return result, fmt.Errorf("read services: %w", err)
	}
	if result.Services == nil {
		return result, fmt.Errorf("services must be an array (use services: [] to clear)")
	}
	return result, nil
}

func printRaw(cmd *cobra.Command, raw json.RawMessage) error {
	_, err := fmt.Fprintln(cmd.OutOrStdout(), string(raw))
	return err
}
func printAgents(cmd *cobra.Command, agents []dto.AgentStatusResponse) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tPROJECT")
	for _, a := range agents {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", a.ID, tableText(a.Name), tableText(a.Status), tableText(a.ProjectName))
	}
	return w.Flush()
}
