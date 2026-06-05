package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/api/http/middleware"
	"github.com/lwlee2608/overwatcher/internal/service/agentregistry"
	"github.com/lwlee2608/overwatcher/internal/service/project"
)

type AgentHandler struct {
	agentService   *agentregistry.Service
	projectService *project.Service
}

func NewAgentHandler(svc *agentregistry.Service, projectSvc *project.Service) *AgentHandler {
	return &AgentHandler{agentService: svc, projectService: projectSvc}
}

// canManage reports whether callerID may see and manage the agent, and writes
// a 404 (deliberately not 403 — don't leak existence) when not. Visibility
// follows the agent lifecycle: an unbound agent belongs to its installer; once
// bound, to the members of its project.
func (h *AgentHandler) canManage(c *gin.Context, callerID string, a *agentregistry.AgentStatus) bool {
	if a.ProjectID != "" {
		if _, err := h.projectService.Access(c.Request.Context(), callerID, a.ProjectID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return false
		}
		return true
	}
	if a.InstalledBy == callerID {
		return true
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
	return false
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

// Create mints the agent's token. The raw token is in the response body once
// only — never retrievable again.
func (h *AgentHandler) Create(c *gin.Context) {
	var req dto.CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	agentID, token, err := h.agentService.Create(c.Request.Context(), req.Name, callerID, req.Token)
	if err != nil {
		if errors.Is(err, agentregistry.ErrNameTaken) {
			c.JSON(http.StatusConflict, gin.H{"error": "an agent with that name already exists"})
			return
		}
		if errors.Is(err, agentregistry.ErrBadToken) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "malformed agent token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, dto.AgentTokenResponse{AgentID: agentID, Name: req.Name, Token: token})
}

// MintToken issues a fresh agent token without persisting an agent; the row is
// created on Create, which is handed this same token. See Service.MintToken.
func (h *AgentHandler) MintToken(c *gin.Context) {
	if _, ok := middleware.UserID(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	token, err := h.agentService.MintToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.AgentTokenResponse{Token: token})
}

// ReissueToken replaces the agent's token — for migrating pre-token agents and
// recovering a lost token.
func (h *AgentHandler) ReissueToken(c *gin.Context) {
	id := c.Param("id")
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	a, err := h.agentService.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !h.canManage(c, callerID, a) {
		return
	}
	token, err := h.agentService.ReissueToken(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, dto.AgentTokenResponse{AgentID: a.ID, Name: a.Name, Token: token})
}

func (h *AgentHandler) List(c *gin.Context) {
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	agents, err := h.agentService.ListForUser(c.Request.Context(), callerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := dto.AgentListResponse{
		Agents: make([]dto.AgentStatusResponse, len(agents)),
	}
	for i, a := range agents {
		resp.Agents[i] = toAgentResponse(&a)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *AgentHandler) Get(c *gin.Context) {
	id := c.Param("id")
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	a, err := h.agentService.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !h.canManage(c, callerID, a) {
		return
	}
	c.JSON(http.StatusOK, toAgentResponse(a))
}

func (h *AgentHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	a, err := h.agentService.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !h.canManage(c, callerID, a) {
		return
	}
	if err := h.agentService.Delete(c.Request.Context(), id); err != nil {
		switch {
		case errors.Is(err, agentregistry.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		case errors.Is(err, agentregistry.ErrBound):
			c.JSON(http.StatusConflict, gin.H{"error": "agent is bound to a project; unbind first"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// BindProject sets (or clears, when project_id is empty) the agent's project binding.
//
// Authorization: rebinding moves future deploys to a different compose file. The
// caller must be able to manage the source side — own the project the agent is
// currently bound to, or be the installer of an unbound agent — and own the new
// target. Owning only the target would let any owner steal an agent away from
// another team's project.
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
	} else if current.InstalledBy != callerID {
		// Unbound agent: only its installer may bind it.
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
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
	c.JSON(http.StatusOK, toAgentResponse(a))
}

func toAgentResponse(a *agentregistry.AgentStatus) dto.AgentStatusResponse {
	return dto.AgentStatusResponse{
		ID:          a.ID,
		Name:        a.Name,
		LastSeen:    a.LastSeen,
		RemoteIP:    a.RemoteIP,
		Status:      string(a.Status),
		ProjectID:   a.ProjectID,
		ProjectName: a.ProjectName,
		Type:        a.Type,
		Version:     a.Version,
	}
}
