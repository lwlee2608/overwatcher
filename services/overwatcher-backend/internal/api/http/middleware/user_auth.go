package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lwlee2608/overwatcher/internal/service/apikey"
	"github.com/lwlee2608/overwatcher/internal/service/auth"
)

const ContextAPIKeyAuthKey = "auth.api_key"

// UserAuth authenticates a user by `Authorization: Bearer owk_…` API key or,
// when no Authorization header is sent, by session cookie. A present but
// invalid header is a 401 — it never falls back to the cookie.
func UserAuth(sessions *auth.Service, keys *apikey.Service, cfg CookieConfig) gin.HandlerFunc {
	cookieAuth := SessionAuth(sessions, cfg)
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			cookieAuth(c)
			return
		}
		raw, ok := strings.CutPrefix(header, "Bearer ")
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed bearer token"})
			return
		}
		userID, err := keys.Resolve(c.Request.Context(), raw)
		if err != nil {
			if errors.Is(err, apikey.ErrNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
				return
			}
			slog.Error("api key resolve failed", "error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		c.Set(ContextUserIDKey, userID)
		c.Set(ContextAPIKeyAuthKey, true)
		c.Next()
	}
}

// RequireSession rejects API-key callers. Guards credential management so a
// leaked key can't mint more keys or change the user's password.
func RequireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetBool(ContextAPIKeyAuthKey) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "requires a login session, not an api key"})
			return
		}
		c.Next()
	}
}
