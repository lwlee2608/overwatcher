package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/service/user"
)

type UserHandler struct {
	svc *user.Service
}

func NewUserHandler(svc *user.Service) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) List(c *gin.Context) {
	entries, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp := dto.UserListResponse{Users: make([]dto.UserResponse, len(entries))}
	for i, e := range entries {
		resp.Users[i] = userToDTO(e)
	}
	c.JSON(http.StatusOK, resp)
}

func (h *UserHandler) Get(c *gin.Context) {
	e, err := h.svc.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, userToDTO(*e))
}

func (h *UserHandler) Create(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	e, err := h.svc.Create(c.Request.Context(), user.CreateParams{
		Email:    req.Email,
		Name:     req.Name,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, user.ErrPasswordTooShort) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "email already in use"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, userToDTO(*e))
}

func (h *UserHandler) Update(c *gin.Context) {
	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	e, err := h.svc.Update(c.Request.Context(), c.Param("id"), user.UpdateParams{
		Email: req.Email,
		Name:  req.Name,
	})
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "email already in use"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, userToDTO(*e))
}

func (h *UserHandler) Delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, user.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func userToDTO(e user.Entry) dto.UserResponse {
	return dto.UserResponse{
		ID:        e.ID,
		Email:     e.Email,
		Name:      e.Name,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
