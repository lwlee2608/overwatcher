package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/api/http/middleware"
	"github.com/lwlee2608/overwatcher/internal/service/apikey"
)

type APIKeyHandler struct {
	svc *apikey.Service
}

func NewAPIKeyHandler(svc *apikey.Service) *APIKeyHandler {
	return &APIKeyHandler{svc: svc}
}

func (h *APIKeyHandler) List(c *gin.Context) {
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	keys, err := h.svc.List(c.Request.Context(), callerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := dto.APIKeyListResponse{Keys: make([]dto.APIKeyResponse, len(keys))}
	for i, k := range keys {
		resp.Keys[i] = apiKeyToDTO(k)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *APIKeyHandler) Create(c *gin.Context) {
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	var req dto.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	k, raw, err := h.svc.Create(c.Request.Context(), callerID, req.Name)
	if err != nil {
		switch {
		case errors.Is(err, apikey.ErrNameMissing):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, apikey.ErrNameTaken):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, dto.CreateAPIKeyResponse{APIKeyResponse: apiKeyToDTO(*k), Key: raw})
}

func (h *APIKeyHandler) Delete(c *gin.Context) {
	callerID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), callerID, c.Param("id")); err != nil {
		if errors.Is(err, apikey.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "api key not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func apiKeyToDTO(k apikey.APIKey) dto.APIKeyResponse {
	return dto.APIKeyResponse{
		ID:         k.ID,
		Name:       k.Name,
		LastUsedAt: k.LastUsedAt,
		CreatedAt:  k.CreatedAt,
	}
}
