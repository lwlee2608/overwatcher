package tests

import (
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

func TestAgents(t *testing.T, router *gin.Engine, agentSvc *agent.Service) {
	t.Run("ListEmpty", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/agents", nil)
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var resp dto.AgentListResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Empty(t, resp.Agents)
	})

	// Seed an agent via the service so we can query it
	err := agentSvc.Record(context.Background(), "test-agent", "10.0.0.1")
	require.NoError(t, err)

	t.Run("ListAfterRecord", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/agents", nil)
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var resp dto.AgentListResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		require.Len(t, resp.Agents, 1)
		assert.Equal(t, "test-agent", resp.Agents[0].Name)
		assert.Equal(t, "10.0.0.1", resp.Agents[0].RemoteIP)
		assert.True(t, resp.Agents[0].Connected)
	})

	t.Run("GetByID", func(t *testing.T) {
		// First get the agent ID from the list
		agents, err := agentSvc.List(context.Background())
		require.NoError(t, err)
		require.Len(t, agents, 1)
		agentID := agents[0].ID

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/agents/"+agentID, nil)
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var resp dto.AgentStatusResponse
		err = json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, agentID, resp.ID)
		assert.Equal(t, "test-agent", resp.Name)
	})

	t.Run("GetNotFound", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/agents/00000000-0000-0000-0000-000000000000", nil)
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}
