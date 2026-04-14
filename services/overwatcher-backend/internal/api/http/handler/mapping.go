package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/service/mapping"
)

type MappingHandler struct {
	mappingService *mapping.Service
}

func NewMappingHandler(svc *mapping.Service) *MappingHandler {
	return &MappingHandler{mappingService: svc}
}

// List returns all deploy mappings.
func (h *MappingHandler) List(c *gin.Context) {
	entries, err := h.mappingService.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := dto.DeployMappingListResponse{
		Mappings: make([]dto.DeployMappingResponse, len(entries)),
	}
	for i, e := range entries {
		resp.Mappings[i] = entryToResponse(e)
	}
	c.JSON(http.StatusOK, resp)
}

// Create creates a new deploy mapping.
func (h *MappingHandler) Create(c *gin.Context) {
	var req dto.CreateDeployMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.Services == nil {
		req.Services = []string{}
	}

	entry, err := h.mappingService.Create(c.Request.Context(), mapping.CreateParams{
		Repo:        req.Repo,
		AgentID:     req.AgentID,
		Services:    req.Services,
		Environment: req.Environment,
		Enabled:     enabled,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, entryToResponse(*entry))
}

// Update modifies an existing deploy mapping.
func (h *MappingHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req dto.UpdateDeployMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if req.Services == nil {
		req.Services = []string{}
	}

	entry, err := h.mappingService.Update(c.Request.Context(), id, mapping.UpdateParams{
		Repo:        req.Repo,
		AgentID:     req.AgentID,
		Services:    req.Services,
		Environment: req.Environment,
		Enabled:     enabled,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "mapping not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entryToResponse(*entry))
}

// Delete removes a deploy mapping.
func (h *MappingHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.mappingService.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func entryToResponse(e mapping.Entry) dto.DeployMappingResponse {
	return dto.DeployMappingResponse{
		ID:          e.ID,
		Repo:        e.Repo,
		AgentID:     e.AgentID,
		AgentName:   e.AgentName,
		Services:    e.Services,
		Environment: e.Environment,
	}
}
