package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/client"
	"github.com/stretchr/testify/require"
)

func testOwctlManagement(t *testing.T, router *gin.Engine, sessionToken string, c *client.Client, binary, home, serverURL, key string, run func(string, ...string) (string, error)) {
	ctx := context.Background()
	output, err := run(key, "project", "create", "owctl-managed", "--compose-file", "/srv/compose.yml", "--environment", "staging", "--description", "CLI project", "--enabled=false", "--json")
	require.NoError(t, err, output)
	var project dto.ProjectResponse
	require.NoError(t, json.Unmarshal([]byte(output), &project))
	require.Equal(t, "owctl-managed", project.Name)
	require.False(t, project.Enabled)
	require.Equal(t, "staging", project.Environment)
	defer doJSON(t, router, "DELETE", "/api/v1/projects/"+project.ID, nil, sessionToken)
	output, err = run(key, "project", "get", project.Name, "--json")
	require.NoError(t, err, output)
	require.Contains(t, output, project.ID)
	yamlFile := filepath.Join(t.TempDir(), "services.yaml")
	require.NoError(t, os.WriteFile(yamlFile, []byte("services:\n  - name: web\n    repo: owner/web\n    image: ghcr.io/owner/web\n    root_directory: app\n    branch: main\n    tag: latest\n    workflow: build.yml\n    position: 3\n"), 0600))
	output, err = run(key, "service", "set", project.Name, "-f", yamlFile, "--json")
	require.NoError(t, err, output)
	var services dto.ComposeServiceListResponse
	require.NoError(t, json.Unmarshal([]byte(output), &services))
	require.Len(t, services.Services, 1)
	require.Equal(t, "app", services.Services[0].RootDirectory)
	require.Equal(t, 0, services.Services[0].Position)
	got, err := c.GetProject(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, got.Data.Services, 1)
	jsonFile := filepath.Join(t.TempDir(), "services.json")
	require.NoError(t, os.WriteFile(jsonFile, []byte(`{"services":[{"name":"worker","repo":"owner/worker","image":"worker"}]}`), 0600))
	output, err = run(key, "service", "set", project.ID, "-f", jsonFile)
	require.NoError(t, err, output)
	require.Contains(t, output, "worker")
	require.NotContains(t, output, "web")
	stdin := exec.Command(binary, "service", "set", project.Name, "-f", "-", "--json")
	stdin.Env = append(os.Environ(), "HOME="+home, "OVERWATCHER_URL="+serverURL, "OVERWATCHER_API_KEY="+key)
	stdin.Stdin = strings.NewReader(`{"services":[]}`)
	data, err := stdin.CombinedOutput()
	require.NoError(t, err, string(data))
	require.JSONEq(t, `{"services":[]}`, string(data))
	_, err = c.ReplaceServices(ctx, project.ID, dto.ReplaceComposeServicesRequest{Services: []dto.CreateComposeServiceRequest{{Name: "api", Repo: "owner/api", Image: "api"}}})
	require.NoError(t, err)
	rr := doJSON(t, router, "POST", "/api/v1/agents", []byte(`{"name":"owctl-existing"}`), sessionToken)
	require.Equal(t, http.StatusCreated, rr.Code)
	var agent dto.AgentTokenResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &agent))
	defer func() {
		doJSON(t, router, "PUT", "/api/v1/agents/"+agent.AgentID+"/project", []byte(`{"project_id":""}`), sessionToken)
		doJSON(t, router, "DELETE", "/api/v1/agents/"+agent.AgentID, nil, sessionToken)
	}()
	agents, err := c.ListAgents(ctx)
	require.NoError(t, err)
	output, err = run(key, "agent", "list", "--json")
	require.NoError(t, err, output)
	require.JSONEq(t, string(agents.Raw), output)
	require.NotContains(t, output, agent.Token)
	output, err = run(key, "agent", "list")
	require.NoError(t, err, output)
	require.Contains(t, output, agent.AgentID)
	output, err = run(key, "agent", "bind", agent.AgentID, project.Name, "--json")
	require.NoError(t, err, output)
	var bound dto.AgentStatusResponse
	require.NoError(t, json.Unmarshal([]byte(output), &bound))
	require.Equal(t, project.ID, bound.ProjectID)
	require.NotContains(t, output, agent.Token)
	output, err = run(key, "agent", "bind", agent.AgentID, project.ID)
	require.NoError(t, err, output)
	require.Contains(t, output, project.Name)
	_, err = c.BindAgent(ctx, agent.AgentID, dto.BindAgentProjectRequest{})
	require.NoError(t, err)
	output, err = run(key, "project", "delete", project.Name, "--json")
	require.NoError(t, err, output)
	require.Empty(t, output)
	output, err = run(key, "project", "get", project.Name)
	require.Error(t, err)
	require.Contains(t, output, "project not found")
	output, err = run(key, "project", "get", project.ID)
	require.Error(t, err)
	require.Contains(t, output, "project not found")
	created, err := c.CreateProject(ctx, dto.CreateProjectRequest{Name: "owctl-client-managed", ComposeFile: "/srv/compose.yml"})
	require.NoError(t, err)
	defer doJSON(t, router, "DELETE", "/api/v1/projects/"+created.Data.ID, nil, sessionToken)
	output, err = run(key, "project", "delete", created.Data.ID)
	require.NoError(t, err, output)
	require.Contains(t, output, "Project deleted")
}
