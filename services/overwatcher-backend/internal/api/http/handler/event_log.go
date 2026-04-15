package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/service/eventlog"
)

type EventLogHandler struct {
	eventLogService *eventlog.Service
}

func NewEventLogHandler(svc *eventlog.Service) *EventLogHandler {
	return &EventLogHandler{eventLogService: svc}
}

func (h *EventLogHandler) List(c *gin.Context) {
	limit := int32(50)
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = int32(n)
		}
	}

	events, err := h.eventLogService.List(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := dto.EventLogListResponse{
		Events: make([]dto.EventLogResponse, len(events)),
	}
	for i, e := range events {
		resp.Events[i] = dto.EventLogResponse{
			ID:         e.ID,
			DeliveryID: e.DeliveryID,
			EventType:  e.EventType,
			Repo:       e.Repo,
			Sender:     e.Sender,
			Summary:    e.Summary,
			CreatedAt:  e.CreatedAt,
		}
	}
	c.JSON(http.StatusOK, resp)
}
