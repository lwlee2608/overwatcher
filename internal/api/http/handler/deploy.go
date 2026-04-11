package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/service"
)

type DeployHandler struct {
	dispatchService *service.DispatchService
}

func NewDeployHandler(dispatchService *service.DispatchService) *DeployHandler {
	return &DeployHandler{dispatchService: dispatchService}
}

// Next is a long-poll. It blocks inside DispatchService.Next until an intent
// is available or the request context is cancelled (client disconnect or
// server shutdown). On context cancellation it returns 204 so the agent can
// re-poll cleanly without treating it as an error.
func (h *DeployHandler) Next(c *gin.Context) {
	intent, err := h.dispatchService.Next(c.Request.Context())
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			c.Status(http.StatusNoContent)
			return
		}
		slog.Error("dispatch Next failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "dispatch failed"})
		return
	}

	c.JSON(http.StatusOK, dto.DeployIntentResponse{
		ID:           intent.ID,
		CreatedAt:    intent.CreatedAt,
		DeliveryID:   intent.DeliveryID,
		Repo:         intent.Repo,
		Ref:          intent.Ref,
		SHA:          intent.SHA,
		Image:        intent.Image,
		Tag:          intent.Tag,
		Stack:        intent.Stack,
		Services:     intent.Services,
		Environment:  intent.Environment,
		DeploymentID: intent.DeploymentID,
	})
}

// Result records an agent's deploy outcome. Returns 404 for unknown intent
// IDs (e.g. coordinator restarted between take and report).
func (h *DeployHandler) Result(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var req dto.DeployResultRequest
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
