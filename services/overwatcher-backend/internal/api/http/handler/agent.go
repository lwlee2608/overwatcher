package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/api/http/middleware"
	"github.com/lwlee2608/overwatcher/internal/service/agent"
	"github.com/lwlee2608/overwatcher/internal/service/project"
)

type AgentHandler struct {
	agentService   *agent.Service
	projectService *project.Service
}

func NewAgentHandler(svc *agent.Service, projectSvc *project.Service) *AgentHandler {
	return &AgentHandler{agentService: svc, projectService: projectSvc}
}

// requireOwner returns false (and writes the response) when callerID is not
// the owner of projectID.
func (h *AgentHandler) requireOwner(c *gin.Context, callerID, projectID string) bool {
	role, err := h.projectService.Access(c.Request.Context(), callerID, projectID)
	if err != nil {
		switch {
		case errors.Is(err, project.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		case errors.Is(err, project.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "owner only"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return false
	}
	if role != project.RoleOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "owner only"})
		return false
	}
	return true
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
//
// Authorization: rebinding moves future deploys to a different compose file,
// so the caller must own BOTH endpoints — the project the agent is currently
// bound to (if any) and the new target (if any). Checking only the target
// would let any owner steal an agent away from another team's project.
func (h *AgentHandler) BindProject(c *gin.Context) {
	id := c.Param("id")
	var req dto.BindAgentProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	current, err := h.agentService.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if current.ProjectID != "" {
		if !h.requireOwner(c, callerID, current.ProjectID) {
			return
		}
	}
	if req.ProjectID != "" && req.ProjectID != current.ProjectID {
		if !h.requireOwner(c, callerID, req.ProjectID) {
			return
		}
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
