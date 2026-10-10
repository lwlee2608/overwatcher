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
	"github.com/lwlee2608/overwatcher/internal/service/apikey"
)

func TestAPIKeys(t *testing.T, router *gin.Engine, sessionToken string) {
	var keyID, rawKey string

	t.Run("CreateRequiresAuth", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateAPIKeyRequest{Name: "ci-bot"})
		rr := doJSON(t, router, "POST", "/api/v1/api-keys", body, "")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Create", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateAPIKeyRequest{Name: "ci-bot"})
		rr := doJSON(t, router, "POST", "/api/v1/api-keys", body, sessionToken)
		require.Equal(t, http.StatusCreated, rr.Code)
		var resp dto.CreateAPIKeyResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		assert.Equal(t, "ci-bot", resp.Name)
		assert.Nil(t, resp.LastUsedAt)
		assert.Contains(t, resp.Key, apikey.TokenPrefix)
		keyID, rawKey = resp.ID, resp.Key
	})

	t.Run("CreateDuplicateName", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateAPIKeyRequest{Name: "ci-bot"})
		rr := doJSON(t, router, "POST", "/api/v1/api-keys", body, sessionToken)
		assert.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("CreateBlankName", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateAPIKeyRequest{Name: "   "})
		rr := doJSON(t, router, "POST", "/api/v1/api-keys", body, sessionToken)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("ListOmitsRawKey", func(t *testing.T) {
		rr := doJSON(t, router, "GET", "/api/v1/api-keys", nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.NotContains(t, rr.Body.String(), rawKey)
		var resp dto.APIKeyListResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		require.Len(t, resp.Keys, 1)
		assert.Equal(t, keyID, resp.Keys[0].ID)
	})

	t.Run("BearerActsAsUser", func(t *testing.T) {
		rr := doBearer(t, router, "GET", "/api/v1/auth/me", nil, rawKey)
		require.Equal(t, http.StatusOK, rr.Code)
		var me dto.MeResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&me))
		assert.Equal(t, "test@example.com", me.Email)
	})

	t.Run("BearerCreatesProjectWithServices", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateProjectRequest{Name: "bot-project", ComposeFile: "/opt/stacks/bot/docker-compose.yml"})
		rr := doBearer(t, router, "POST", "/api/v1/projects", body, rawKey)
		require.Equal(t, http.StatusCreated, rr.Code)
		var p dto.ProjectResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&p))
		assert.Equal(t, "test@example.com", p.UserEmail)

		body, _ = json.Marshal(dto.ReplaceComposeServicesRequest{Services: []dto.CreateComposeServiceRequest{
			{Name: "web", Repo: "acme/web", Image: "acme/web"},
		}})
		rr = doBearer(t, router, "PUT", "/api/v1/projects/"+p.ID+"/services", body, rawKey)
		require.Equal(t, http.StatusOK, rr.Code)

		rr = doJSON(t, router, "GET", "/api/v1/projects/"+p.ID, nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var got dto.ProjectResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
		assert.Equal(t, "owner", got.Role)
		require.Len(t, got.Services, 1)
	})

	t.Run("LastUsedRecorded", func(t *testing.T) {
		rr := doJSON(t, router, "GET", "/api/v1/api-keys", nil, sessionToken)
		require.Equal(t, http.StatusOK, rr.Code)
		var resp dto.APIKeyListResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		require.Len(t, resp.Keys, 1)
		assert.NotNil(t, resp.Keys[0].LastUsedAt)
	})

	t.Run("BearerCannotManageKeys", func(t *testing.T) {
		rr := doBearer(t, router, "GET", "/api/v1/api-keys", nil, rawKey)
		assert.Equal(t, http.StatusForbidden, rr.Code)
		body, _ := json.Marshal(dto.CreateAPIKeyRequest{Name: "escalate"})
		rr = doBearer(t, router, "POST", "/api/v1/api-keys", body, rawKey)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("BearerCannotManageUsers", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateUserRequest{Email: "backdoor@example.com", Password: "backdoor-pass"})
		rr := doBearer(t, router, "POST", "/api/v1/users", body, rawKey)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("BearerCannotChangePassword", func(t *testing.T) {
		body, _ := json.Marshal(dto.ChangePasswordRequest{OldPassword: "testpassword", NewPassword: "hijacked-pass"})
		rr := doBearer(t, router, "PUT", "/api/v1/auth/password", body, rawKey)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("InvalidBearer", func(t *testing.T) {
		rr := doBearer(t, router, "GET", "/api/v1/projects", nil, apikey.TokenPrefix+"bogus")
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("InvalidBearerDoesNotFallBackToCookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/projects", nil)
		req.Header.Set("Authorization", "Bearer "+apikey.TokenPrefix+"bogus")
		setSession(req, sessionToken)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("AgentTokenRejected", func(t *testing.T) {
		rr := doBearer(t, router, "GET", "/api/v1/projects", nil, TestAgentToken)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("Delete", func(t *testing.T) {
		rr := doJSON(t, router, "DELETE", "/api/v1/api-keys/"+keyID, nil, sessionToken)
		require.Equal(t, http.StatusNoContent, rr.Code)

		rr = doBearer(t, router, "GET", "/api/v1/projects", nil, rawKey)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)

		rr = doJSON(t, router, "DELETE", "/api/v1/api-keys/"+keyID, nil, sessionToken)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("DeleteMalformedID", func(t *testing.T) {
		rr := doJSON(t, router, "DELETE", "/api/v1/api-keys/not-a-uuid", nil, sessionToken)
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func doBearer(t *testing.T, router *gin.Engine, method, path string, body []byte, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}
