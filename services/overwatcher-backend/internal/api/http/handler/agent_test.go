package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/service/agent"
)

func newAgentTestRouter(tracker *agent.Tracker) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewAgentHandler(tracker)
	r.GET("/api/v1/agents", h.List)
	return r
}

func TestAgentHandler_List_Empty(t *testing.T) {
	tracker := agent.NewTracker(60 * time.Second)
	r := newAgentTestRouter(tracker)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp dto.AgentListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(resp.Agents))
	}
}

func TestAgentHandler_List_WithAgents(t *testing.T) {
	tracker := agent.NewTracker(60 * time.Second)
	tracker.Record("vm-1", []string{"web", "api"}, "10.0.0.1")
	tracker.Record("vm-2", []string{"worker"}, "10.0.0.2")

	r := newAgentTestRouter(tracker)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp dto.AgentListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(resp.Agents))
	}
	if resp.Agents[0].Name != "vm-1" {
		t.Errorf("first agent = %q, want %q", resp.Agents[0].Name, "vm-1")
	}
	if !resp.Agents[0].Connected {
		t.Error("expected vm-1 to be connected")
	}
}
