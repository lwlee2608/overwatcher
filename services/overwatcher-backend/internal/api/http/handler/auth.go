package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/lwlee2608/overwatcher/internal/api/http/middleware"
	"github.com/lwlee2608/overwatcher/internal/service/auth"
)

type AuthHandler struct {
	svc       *auth.Service
	cookieCfg middleware.CookieConfig
}

func NewAuthHandler(svc *auth.Service, cookieCfg middleware.CookieConfig) *AuthHandler {
	return &AuthHandler{svc: svc, cookieCfg: cookieCfg}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sess, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		slog.Error("login failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	user, err := h.svc.GetUser(c.Request.Context(), sess.UserID)
	if err != nil {
		slog.Error("login user lookup failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	middleware.SetSessionCookie(c, sess.Token, sess.ExpiresAt, h.cookieCfg)
	c.JSON(http.StatusOK, dto.MeResponse{
		ID:                 user.ID,
		Email:              user.Email,
		Name:               user.Name,
		MustChangePassword: user.PasswordIsBootstrap,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if token, err := c.Cookie(middleware.SessionCookieName); err == nil && token != "" {
		if err := h.svc.Logout(c.Request.Context(), token); err != nil {
			slog.Error("logout failed", "error", err)
		}
	}
	middleware.ClearSessionCookie(c, h.cookieCfg)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	user, err := h.svc.GetUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		slog.Error("me user lookup failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, dto.MeResponse{
		ID:                 user.ID,
		Email:              user.Email,
		Name:               user.Name,
		MustChangePassword: user.PasswordIsBootstrap,
	})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, ok := middleware.UserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "current password incorrect"})
		case errors.Is(err, auth.ErrPasswordTooShort):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			slog.Error("change password failed", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}
