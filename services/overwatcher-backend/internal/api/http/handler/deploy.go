package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/api/http/middleware"
	"github.com/lwlee2608/overwatcher/internal/protocol"
	"github.com/lwlee2608/overwatcher/internal/service/dispatch"
	"github.com/lwlee2608/overwatcher/internal/service/intent"
	"github.com/lwlee2608/overwatcher/internal/service/project"
	"github.com/lwlee2608/overwatcher/internal/service/webhook"
)

// LongPollTimeout caps how long Next holds an idle request. Sits comfortably
// under typical 30s proxy/LB idle timeouts and the agent's 30s client timeout
// so the 204 always reaches the agent before either fires. var, not const,
// so tests can shrink it.
var LongPollTimeout = 25 * time.Second

type DeployHandler struct {
	dispatchService *dispatch.Service
	webhookService  *webhook.Service
	projectService  *project.Service
}

func NewDeployHandler(dispatchService *dispatch.Service, webhookService *webhook.Service, projectService *project.Service) *DeployHandler {
	return &DeployHandler{dispatchService: dispatchService, webhookService: webhookService, projectService: projectService}
}

// Next is a long-poll. It blocks inside DispatchService.Next for up to
// longPollTimeout, or until the request context is cancelled (agent
// disconnect). On either, it returns 204 so the agent can re-poll cleanly
// without treating it as a transport error. X-Agent-Name scopes the claim to
// the project bound to that agent.
func (h *DeployHandler) Next(c *gin.Context) {
	agentName := c.GetHeader("X-Agent-Name")
	if agentName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing X-Agent-Name header"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), LongPollTimeout)
	defer cancel()

	intentRow, err := h.dispatchService.Next(ctx, agentName)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			c.Status(http.StatusNoContent)
			return
		}
		slog.Error("dispatch Next failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "dispatch failed"})
		return
	}

	c.JSON(http.StatusOK, protocol.DeployIntentResponse{
		ID:           intentRow.ID,
		CreatedAt:    intentRow.CreatedAt,
		DeliveryID:   intentRow.DeliveryID,
		ProjectID:    intentRow.ProjectID,
		ComposeFile:  intentRow.ComposeFile,
		Repo:         intentRow.Repo,
		Ref:          intentRow.Ref,
		SHA:          intentRow.SHA,
		Stack:        intentRow.Stack,
		Services:     intentServicesToDTO(intentRow.Services),
		Environment:  intentRow.Environment,
		DeploymentID: intentRow.DeploymentID,
	})
}

func (h *DeployHandler) ListDeployments(c *gin.Context) {
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	limit := int32(50)
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = int32(n)
		}
	}

	intents, err := h.dispatchService.ListRecentForUser(c.Request.Context(), callerID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := dto.DeploymentListResponse{
		Deployments: make([]dto.DeploymentResponse, len(intents)),
	}
	for i, d := range intents {
		resp.Deployments[i] = dto.DeploymentResponse{
			ID:          d.ID,
			CreatedAt:   d.CreatedAt,
			DeliveryID:  d.DeliveryID,
			ProjectID:   d.ProjectID,
			Repo:        d.Repo,
			Ref:         d.Ref,
			SHA:         d.SHA,
			Stack:       d.Stack,
			Services:    intentServicesToDTO(d.Services),
			Environment: d.Environment,
			Status:      string(d.Status),
			Attempts:    d.Attempts,
		}
	}
	c.JSON(http.StatusOK, resp)
}

// Redeploy clones an existing deploy intent into a new one so the agent
// re-runs the same stack/SHA/services without requiring a fresh git push.
//
// Authorization: the caller must own the source intent's project or be a
// member. Without this check any logged-in user could redeploy any project's
// stack just by guessing intent IDs.
func (h *DeployHandler) Redeploy(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	source, err := h.dispatchService.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, intent.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if source.ProjectID == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "source deployment is not redeployable"})
		return
	}
	if _, err := h.projectService.Access(c.Request.Context(), callerID, source.ProjectID); err != nil {
		switch {
		case errors.Is(err, project.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		case errors.Is(err, project.ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	if err := h.webhookService.Redeploy(c.Request.Context(), id); err != nil {
		slog.Error("Manual redeploy failed", "source_id", id, "error", err)
		switch {
		case errors.Is(err, intent.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		case errors.Is(err, webhook.ErrNoInstallation), errors.Is(err, webhook.ErrInvalidRepo):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "source deployment is not redeployable"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "redeploy failed"})
		}
		return
	}
	c.Status(http.StatusAccepted)
}

func intentServicesToDTO(specs []intent.ServiceSpec) []protocol.ServiceSpecDTO {
	out := make([]protocol.ServiceSpecDTO, len(specs))
	for i, s := range specs {
		out[i] = protocol.ServiceSpecDTO{Name: s.Name, Image: s.Image, Tag: s.Tag}
	}
	return out
}

// Result records an agent's deploy outcome. Returns 404 for unknown intent
// IDs (e.g. coordinator restarted between take and report).
func (h *DeployHandler) Result(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var req protocol.DeployResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	switch req.State {
	case "success", "failure":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "state must be 'success' or 'failure'"})
		return
	}

	if !h.dispatchService.Report(c.Request.Context(), id, req.State == "success", req.Error) {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown intent id"})
		return
	}
	c.Status(http.StatusNoContent)
}
