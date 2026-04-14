package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/service/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMappings(t *testing.T, router *gin.Engine, agentSvc *agent.Service) {
	// Get the agent ID seeded by TestAgents
	agents, err := agentSvc.List(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, agents)
	agentID := agents[0].ID

	t.Run("ListEmpty", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/mappings", nil)
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var resp dto.DeployMappingListResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Empty(t, resp.Mappings)
	})

	var createdID string

	t.Run("Create", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateDeployMappingRequest{
			Repo:        "lwlee2608/overwatcher",
			AgentID:     agentID,
			Services:    []string{"web", "api"},
			Environment: "staging",
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/mappings", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusCreated, rr.Code)

		var resp dto.DeployMappingResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
		assert.Equal(t, "lwlee2608/overwatcher", resp.Repo)
		assert.Equal(t, agentID, resp.AgentID)
		assert.Equal(t, []string{"web", "api"}, resp.Services)
		assert.Equal(t, "staging", resp.Environment)
		assert.True(t, resp.Enabled)

		createdID = resp.ID
	})

	t.Run("ListAfterCreate", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/mappings", nil)
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var resp dto.DeployMappingListResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		require.Len(t, resp.Mappings, 1)
		assert.Equal(t, createdID, resp.Mappings[0].ID)
	})

	t.Run("Update", func(t *testing.T) {
		enabled := false
		body, _ := json.Marshal(dto.UpdateDeployMappingRequest{
			Repo:        "lwlee2608/overwatcher",
			AgentID:     agentID,
			Services:    []string{"web"},
			Environment: "production",
			Enabled:     &enabled,
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/api/v1/mappings/"+createdID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var resp dto.DeployMappingResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, []string{"web"}, resp.Services)
		assert.Equal(t, "production", resp.Environment)
		assert.False(t, resp.Enabled)
	})

	t.Run("CreateWithInvalidAgent", func(t *testing.T) {
		body, _ := json.Marshal(dto.CreateDeployMappingRequest{
			Repo:    "foo/bar",
			AgentID: "00000000-0000-0000-0000-000000000000",
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/mappings", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("UpdateNotFound", func(t *testing.T) {
		body, _ := json.Marshal(dto.UpdateDeployMappingRequest{
			Repo:    "foo/bar",
			AgentID: agentID,
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/api/v1/mappings/00000000-0000-0000-0000-000000000000", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("Delete", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", "/api/v1/mappings/"+createdID, nil)
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", "/api/v1/mappings/"+createdID, nil)
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("ListAfterDelete", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/mappings", nil)
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var resp dto.DeployMappingListResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Empty(t, resp.Mappings)
	})
}
