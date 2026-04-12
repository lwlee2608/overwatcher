package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/service/webhook"
)

type WebhookHandler struct {
	webhookService *webhook.Service
}

func NewWebhookHandler(webhookService *webhook.Service) *WebhookHandler {
	return &WebhookHandler{webhookService: webhookService}
}

func (h *WebhookHandler) HandleWebhook(c *gin.Context) {
	eventType := c.GetHeader("X-GitHub-Event")
	deliveryID := c.GetHeader("X-GitHub-Delivery")

	payload, exists := c.Get("webhookPayload")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.WebhookResponse{Status: "error", Message: "missing payload"})
		return
	}

	if err := h.webhookService.HandleEvent(c.Request.Context(), eventType, deliveryID, payload.([]byte)); err != nil {
		c.JSON(http.StatusInternalServerError, dto.WebhookResponse{Status: "error", Message: "failed to process event"})
		return
	}

	c.JSON(http.StatusOK, dto.WebhookResponse{Status: "accepted"})
}
