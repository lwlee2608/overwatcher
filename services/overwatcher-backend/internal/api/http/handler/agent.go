package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/service/agent"
)

type AgentHandler struct {
	agentService *agent.Service
}

func NewAgentHandler(svc *agent.Service) *AgentHandler {
	return &AgentHandler{agentService: svc}
}

// List returns all known agents and their connection status.
func (h *AgentHandler) List(c *gin.Context) {
	agents, err := h.agentService.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := dto.AgentListResponse{
		Agents: make([]dto.AgentStatusResponse, len(agents)),
	}
	for i, a := range agents {
		resp.Agents[i] = dto.AgentStatusResponse{
			ID:        a.ID,
			Name:      a.Name,
			LastSeen:  a.LastSeen,
			RemoteIP:  a.RemoteIP,
			Connected: a.Connected,
			ProjectID: a.ProjectID,
		}
	}
	c.JSON(http.StatusOK, resp)
}

// Get returns a single agent by ID.
func (h *AgentHandler) Get(c *gin.Context) {
	id := c.Param("id")
	a, err := h.agentService.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.AgentStatusResponse{
		ID:        a.ID,
		Name:      a.Name,
		LastSeen:  a.LastSeen,
		RemoteIP:  a.RemoteIP,
		Connected: a.Connected,
		ProjectID: a.ProjectID,
	})
}

// BindProject sets (or clears, when project_id is empty) the agent's project binding.
func (h *AgentHandler) BindProject(c *gin.Context) {
	id := c.Param("id")
	var req dto.BindAgentProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a, err := h.agentService.BindProject(c.Request.Context(), id, req.ProjectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.AgentStatusResponse{
		ID:        a.ID,
		Name:      a.Name,
		LastSeen:  a.LastSeen,
		RemoteIP:  a.RemoteIP,
		Connected: a.Connected,
		ProjectID: a.ProjectID,
	})
}
