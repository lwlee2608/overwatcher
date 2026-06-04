package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/service/agentregistry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgents(t *testing.T, router *gin.Engine, agentSvc *agentregistry.Service, sessionUserID, sessionToken string) {
	t.Run("ListEmpty", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/agents", nil)
		setSession(req, sessionToken)
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var resp dto.AgentListResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Empty(t, resp.Agents)
	})

	// Pre-provision an agent installed by the session user so it's visible.
	agentID, token, err := agentSvc.Create(context.Background(), "test-agent", sessionUserID)
	require.NoError(t, err)
	require.NotEmpty(t, agentID)
	assert.Contains(t, token, agentregistry.TokenPrefix)

	t.Run("ListAfterCreate", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/agents", nil)
		setSession(req, sessionToken)
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var resp dto.AgentListResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		require.Len(t, resp.Agents, 1)
		assert.Equal(t, "test-agent", resp.Agents[0].Name)
	})

	t.Run("GetByID", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/agents/"+agentID, nil)
		setSession(req, sessionToken)
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
		setSession(req, sessionToken)
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}
