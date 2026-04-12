package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/service/agent"
)

type AgentHandler struct {
	tracker *agent.Tracker
}

func NewAgentHandler(tracker *agent.Tracker) *AgentHandler {
	return &AgentHandler{tracker: tracker}
}

// List returns all known agents and their connection status.
func (h *AgentHandler) List(c *gin.Context) {
	agents := h.tracker.List()
	resp := dto.AgentListResponse{
		Agents: make([]dto.AgentStatusResponse, len(agents)),
	}
	for i, a := range agents {
		resp.Agents[i] = dto.AgentStatusResponse{
			Name:      a.Name,
			Stacks:    a.Stacks,
			LastSeen:  a.LastSeen,
			RemoteIP:  a.RemoteIP,
			Connected: a.Connected,
		}
	}
	c.JSON(http.StatusOK, resp)
}
