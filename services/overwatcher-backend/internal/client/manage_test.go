package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/stretchr/testify/require"
)

func TestManagementRequests(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer key", r.Header.Get("Authorization"))
		if r.Method == http.MethodDelete {
			require.Equal(t, "/api/v1/projects/id", r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		switch r.URL.Path {
		case "/api/v1/projects":
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "test", body["name"])
			_, _ = w.Write([]byte(`{"id":"id","name":"test","future":true}`))
		case "/api/v1/projects/id/services":
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, []any{}, body["services"])
			_, _ = w.Write([]byte(`{"services":[]}`))
		case "/api/v1/agents/id/project":
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "project-id", body["project_id"])
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"owner only"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c, err := New(server.URL, "key")
	require.NoError(t, err)
	project, err := c.CreateProject(ctx, dto.CreateProjectRequest{Name: "test", ComposeFile: "compose.yml"})
	require.NoError(t, err)
	require.Equal(t, "id", project.Data.ID)
	require.Contains(t, string(project.Raw), `"future":true`)
	_, err = c.ReplaceServices(ctx, "id", dto.ReplaceComposeServicesRequest{Services: []dto.CreateComposeServiceRequest{}})
	require.NoError(t, err)
	_, err = c.BindAgent(ctx, "id", dto.BindAgentProjectRequest{ProjectID: "project-id"})
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	require.Equal(t, "owner only", apiErr.Message)
	require.NoError(t, c.DeleteProject(ctx, "id"))
	for _, id := range []string{"", ".", "..", "a/b", `a\b`} {
		require.Error(t, c.DeleteProject(ctx, id))
		_, err = c.ReplaceServices(ctx, id, dto.ReplaceComposeServicesRequest{})
		require.Error(t, err)
		_, err = c.BindAgent(ctx, id, dto.BindAgentProjectRequest{})
		require.Error(t, err)
	}
}
