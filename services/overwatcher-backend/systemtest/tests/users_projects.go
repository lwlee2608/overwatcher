package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
)

func TestUsers(t *testing.T, router *gin.Engine, sessionToken string) string {
	var createdID string

	t.Run("ListInitial", func(t *testing.T) {
		rr := doJSON(t, router, "GET", "/api/v1/users", nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var resp dto.UserListResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		// Only the bootstrap auth user should exist before Create runs.
		require.Len(t, resp.Users, 1)
		assert.Equal(t, "test@example.com", resp.Users[0].Email)
	})

	t.Run("Create", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateUserRequest{Email: "alice@example.com", Name: "Alice", Password: "alice-pass-1"})
		rr := doJSON(t, router, "POST", "/api/v1/users", body, sessionToken)
		require.Equal(t, http.StatusCreated, rr.Code)
		var resp dto.UserResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.NotEmpty(t, resp.ID)
		assert.Equal(t, "alice@example.com", resp.Email)
		createdID = resp.ID
	})

	t.Run("CreateDuplicateEmail", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateUserRequest{Email: "alice@example.com", Name: "Alice 2", Password: "alice-pass-2"})
		rr := doJSON(t, router, "POST", "/api/v1/users", body, sessionToken)
		assert.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("Get", func(t *testing.T) {
		rr := doJSON(t, router, "GET", "/api/v1/users/"+createdID, nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Update", func(t *testing.T) {
		body, _ := json.Marshal(dto.UpdateUserRequest{Email: "alice@example.com", Name: "Alice Updated"})
		rr := doJSON(t, router, "PUT", "/api/v1/users/"+createdID, body, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var resp dto.UserResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, "Alice Updated", resp.Name)
	})

	return createdID
}

func TestProjects(t *testing.T, router *gin.Engine, userID string, sessionToken string) {
	var projectID string

	t.Run("CreateProject", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateProjectRequest{
			UserID:      userID,
			Name:        "staging",
			Description: "Alice's staging env",
			ComposeFile: "/srv/compose.yml",
			Environment: "staging",
		})
		rr := doJSON(t, router, "POST", "/api/v1/projects", body, sessionToken)
		require.Equal(t, http.StatusCreated, rr.Code)
		var resp dto.ProjectResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.NotEmpty(t, resp.ID)
		assert.Equal(t, userID, resp.UserID)
		assert.Equal(t, "staging", resp.Name)
		assert.True(t, resp.Enabled)
		projectID = resp.ID
	})

	t.Run("CreateProjectDuplicate", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateProjectRequest{
			UserID:      userID,
			Name:        "staging",
			ComposeFile: "/srv/compose.yml",
		})
		rr := doJSON(t, router, "POST", "/api/v1/projects", body, sessionToken)
		assert.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("CreateProjectBadUser", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateProjectRequest{
			UserID:      "00000000-0000-0000-0000-000000000000",
			Name:        "orphan",
			ComposeFile: "/srv/compose.yml",
		})
		rr := doJSON(t, router, "POST", "/api/v1/projects", body, sessionToken)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("ListProjects", func(t *testing.T) {
		rr := doJSON(t, router, "GET", "/api/v1/projects", nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var resp dto.ProjectListResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		require.Len(t, resp.Projects, 1)
		assert.Equal(t, "alice@example.com", resp.Projects[0].UserEmail)
	})

	t.Run("ListProjectsByUser", func(t *testing.T) {
		rr := doJSON(t, router, "GET", "/api/v1/projects?user_id="+userID, nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var resp dto.ProjectListResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		require.Len(t, resp.Projects, 1)
	})

	t.Run("CreateServices", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateComposeServiceRequest{
			Name:          "web",
			Repo:          "alice/monorepo",
			RootDirectory: "web/",
			Image:         "ghcr.io/alice/web",
		})
		rr := doJSON(t, router, "POST", "/api/v1/projects/"+projectID+"/services", body, sessionToken)
		require.Equal(t, http.StatusCreated, rr.Code)
		var resp dto.ComposeServiceResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, "main", resp.Branch)
		assert.Equal(t, "latest", resp.Tag)
		assert.Equal(t, "web/", resp.RootDirectory)
	})

	t.Run("ReplaceServices", func(t *testing.T) {
		body, _ := json.Marshal(dto.ReplaceComposeServicesRequest{
			Services: []dto.CreateComposeServiceRequest{
				{Name: "web", Repo: "alice/monorepo", RootDirectory: "web/", Image: "ghcr.io/alice/web"},
				{Name: "api", Repo: "alice/monorepo", RootDirectory: "api/", Image: "ghcr.io/alice/api", Tag: "v2"},
			},
		})
		rr := doJSON(t, router, "PUT", "/api/v1/projects/"+projectID+"/services", body, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var resp dto.ComposeServiceListResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		require.Len(t, resp.Services, 2)
		assert.Equal(t, "api", resp.Services[1].Name)
		assert.Equal(t, "v2", resp.Services[1].Tag)
	})

	t.Run("GetProjectIncludesServices", func(t *testing.T) {
		rr := doJSON(t, router, "GET", "/api/v1/projects/"+projectID, nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var resp dto.ProjectResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		require.Len(t, resp.Services, 2)
	})

	t.Run("DeleteProjectCascadesServices", func(t *testing.T) {
		rr := doJSON(t, router, "DELETE", "/api/v1/projects/"+projectID, nil, sessionToken)
		require.Equal(t, http.StatusNoContent, rr.Code)

		rr = doJSON(t, router, "GET", "/api/v1/projects/"+projectID, nil, sessionToken)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func doJSON(t *testing.T, router *gin.Engine, method, path string, body []byte, sessionToken string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	var req *http.Request
	if reader != nil {
		req = httptest.NewRequest(method, path, reader)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	setSession(req, sessionToken)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func setSession(req *http.Request, sessionToken string) {
	if sessionToken == "" {
		return
	}
	req.AddCookie(&http.Cookie{Name: "ow_session", Value: sessionToken})
}
