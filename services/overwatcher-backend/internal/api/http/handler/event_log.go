package handler

import (
	"net/http"

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
	page, pageSize := parsePagination(c)
	filter := eventlog.Filter{
		EventType: c.Query("event_type"),
		Repo:      c.Query("repo"),
		Sender:    c.Query("sender"),
	}

	ctx := c.Request.Context()
	total, err := h.eventLogService.Count(ctx, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	events, err := h.eventLogService.ListPaged(ctx, filter, int32(pageSize), int32((page-1)*pageSize))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := dto.EventLogListResponse{
		Events:   make([]dto.EventLogResponse, len(events)),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
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
