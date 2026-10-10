package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
)

func TestUsers(t *testing.T, router *gin.Engine, sessionToken string) {
	var createdID string

	t.Run("ListInitial", func(t *testing.T) {
		rr := doJSON(t, router, "GET", "/api/v1/users", nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var resp dto.UserListResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		// Only the bootstrap auth user should exist before Create runs.
		require.Len(t, resp.Users, 1)
		assert.Equal(t, "test@example.com", resp.Users[0].Email)
		assert.True(t, resp.Users[0].IsAdmin)
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

	t.Run("MeReportsAdmin", func(t *testing.T) {
		rr := doJSON(t, router, "GET", "/api/v1/auth/me", nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var resp dto.MeResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.True(t, resp.IsAdmin)
	})

	t.Run("NonAdminLimitedToSelf", func(t *testing.T) {
		aliceSession := login(t, router, "alice@example.com", "alice-pass-1")

		body, _ := json.Marshal(dto.CreateUserRequest{Email: "mallory@example.com", Password: "mallory-pass"})
		rr := doJSON(t, router, "POST", "/api/v1/users", body, aliceSession)
		assert.Equal(t, http.StatusForbidden, rr.Code)

		adminID := meID(t, router, sessionToken)
		body, _ = json.Marshal(dto.UpdateUserRequest{Email: "hijack@example.com"})
		rr = doJSON(t, router, "PUT", "/api/v1/users/"+adminID, body, aliceSession)
		assert.Equal(t, http.StatusForbidden, rr.Code)

		rr = doJSON(t, router, "DELETE", "/api/v1/users/"+adminID, nil, aliceSession)
		assert.Equal(t, http.StatusForbidden, rr.Code)

		body, _ = json.Marshal(dto.UpdateUserRequest{Email: "alice@example.com", Name: "Alice Self"})
		rr = doJSON(t, router, "PUT", "/api/v1/users/"+createdID, body, aliceSession)
		require.Equal(t, http.StatusOK, rr.Code)
		var resp dto.UserResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, "Alice Self", resp.Name)
		assert.False(t, resp.IsAdmin)
	})

	t.Run("AdminCannotDeleteSelf", func(t *testing.T) {
		id := meID(t, router, sessionToken)
		rr := doJSON(t, router, "DELETE", "/api/v1/users/"+id, nil, sessionToken)
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		rr = doJSON(t, router, "DELETE", "/api/v1/users/"+strings.ToUpper(strings.ReplaceAll(id, "-", "")), nil, sessionToken)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("TransferProjectsThenDelete", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateUserRequest{Email: "bob@example.com", Password: "bob-pass-1"})
		rr := doJSON(t, router, "POST", "/api/v1/users", body, sessionToken)
		require.Equal(t, http.StatusCreated, rr.Code)
		var bob dto.UserResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&bob))

		bobSession := login(t, router, "bob@example.com", "bob-pass-1")
		body, _ = json.Marshal(dto.CreateProjectRequest{Name: "bob-proj", ComposeFile: "/srv/compose.yml"})
		rr = doJSON(t, router, "POST", "/api/v1/projects", body, bobSession)
		require.Equal(t, http.StatusCreated, rr.Code)
		var proj dto.ProjectResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&proj))

		body, _ = json.Marshal(dto.AddProjectMemberRequest{Email: "test@example.com"})
		rr = doJSON(t, router, "POST", "/api/v1/projects/"+proj.ID+"/members", body, bobSession)
		require.Equal(t, http.StatusCreated, rr.Code)

		rr = doJSON(t, router, "DELETE", "/api/v1/users/"+bob.ID, nil, sessionToken)
		assert.Equal(t, http.StatusConflict, rr.Code)

		adminID := meID(t, router, sessionToken)
		transfer, _ := json.Marshal(dto.TransferProjectsRequest{ToUserID: adminID})
		rr = doJSON(t, router, "POST", "/api/v1/users/"+bob.ID+"/transfer-projects", transfer, bobSession)
		assert.Equal(t, http.StatusForbidden, rr.Code)

		same, _ := json.Marshal(dto.TransferProjectsRequest{ToUserID: bob.ID})
		rr = doJSON(t, router, "POST", "/api/v1/users/"+bob.ID+"/transfer-projects", same, sessionToken)
		assert.Equal(t, http.StatusBadRequest, rr.Code)

		rr = doJSON(t, router, "POST", "/api/v1/users/"+bob.ID+"/transfer-projects", transfer, sessionToken)
		require.Equal(t, http.StatusNoContent, rr.Code)

		rr = doJSON(t, router, "GET", "/api/v1/projects/"+proj.ID, nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var got dto.ProjectResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
		assert.Equal(t, adminID, got.UserID)
		assert.Equal(t, "owner", got.Role)

		rr = doJSON(t, router, "GET", "/api/v1/projects/"+proj.ID+"/members", nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var members dto.ProjectMemberListResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&members))
		for _, m := range members.Members {
			assert.NotEqual(t, "member", m.Role, "new owner should not remain a member")
		}

		rr = doJSON(t, router, "DELETE", "/api/v1/users/"+bob.ID, nil, sessionToken)
		assert.Equal(t, http.StatusNoContent, rr.Code)

		rr = doJSON(t, router, "DELETE", "/api/v1/projects/"+proj.ID, nil, sessionToken)
		require.Equal(t, http.StatusNoContent, rr.Code)
	})
}

func login(t *testing.T, router *gin.Engine, email, password string) string {
	t.Helper()
	body, _ := json.Marshal(dto.LoginRequest{Email: email, Password: password})
	rr := doJSON(t, router, "POST", "/api/v1/auth/login", body, "")
	require.Equal(t, http.StatusOK, rr.Code)
	for _, c := range rr.Result().Cookies() {
		if c.Name == "ow_session" {
			return c.Value
		}
	}
	t.Fatal("login did not set session cookie")
	return ""
}

func meID(t *testing.T, router *gin.Engine, sessionToken string) string {
	t.Helper()
	rr := doJSON(t, router, "GET", "/api/v1/auth/me", nil, sessionToken)
	require.Equal(t, http.StatusOK, rr.Code)
	var me dto.MeResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&me))
	return me.ID
}

func TestProjects(t *testing.T, router *gin.Engine, userID string, sessionToken string) {
	var projectID string

	t.Run("CreateProject", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateProjectRequest{
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
			Name:        "staging",
			ComposeFile: "/srv/compose.yml",
		})
		rr := doJSON(t, router, "POST", "/api/v1/projects", body, sessionToken)
		assert.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("ListProjects", func(t *testing.T) {
		rr := doJSON(t, router, "GET", "/api/v1/projects", nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var resp dto.ProjectListResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		require.Len(t, resp.Projects, 1)
		assert.Equal(t, "test@example.com", resp.Projects[0].UserEmail)
		assert.Equal(t, "owner", resp.Projects[0].Role)
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
