package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/service/agent"
)

func init() { gin.SetMode(gin.TestMode) }

func TestAgentHeartbeat_RecordsFromHeaders(t *testing.T) {
	tracker := agent.NewTracker(60 * time.Second)
	r := gin.New()
	r.Use(AgentHeartbeat(tracker))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Agent-Name", "vm-1")
	req.Header.Set("X-Agent-Stacks", "web, api")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	agents := tracker.List()
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	a := agents[0]
	if a.Name != "vm-1" {
		t.Errorf("name = %q, want %q", a.Name, "vm-1")
	}
	if len(a.Stacks) != 2 || a.Stacks[0] != "web" || a.Stacks[1] != "api" {
		t.Errorf("stacks = %v, want [web api]", a.Stacks)
	}
}

func TestAgentHeartbeat_MissingNameSkips(t *testing.T) {
	tracker := agent.NewTracker(60 * time.Second)
	r := gin.New()
	r.Use(AgentHeartbeat(tracker))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if len(tracker.List()) != 0 {
		t.Error("expected no agents when X-Agent-Name is missing")
	}
}
