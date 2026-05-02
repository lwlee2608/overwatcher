package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lwlee2608/overwatcher/internal/service/auth"
)

const (
	SessionCookieName = "ow_session"
	ContextUserIDKey  = "auth.user_id"
)

type CookieConfig struct {
	Secure bool
	Domain string
}

// SessionAuth requires a valid session cookie. On success, the session's
// user_id is stored in the gin context under ContextUserIDKey, and the
// cookie is refreshed if the underlying session was slid forward.
func SessionAuth(svc *auth.Service, cfg CookieConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(SessionCookieName)
		if err != nil || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}
		sess, err := svc.Validate(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, auth.ErrNoSession) {
				ClearSessionCookie(c, cfg)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
				return
			}
			slog.Error("session validate failed", "error", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		c.Set(ContextUserIDKey, sess.UserID)
		SetSessionCookie(c, sess.Token, sess.ExpiresAt, cfg)
		c.Next()
	}
}

func SetSessionCookie(c *gin.Context, token string, expiresAt time.Time, cfg CookieConfig) {
	maxAge := max(int(time.Until(expiresAt).Seconds()), 0)
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, token, maxAge, "/", cfg.Domain, cfg.Secure, true)
}

func ClearSessionCookie(c *gin.Context, cfg CookieConfig) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(SessionCookieName, "", -1, "/", cfg.Domain, cfg.Secure, true)
}

// UserID pulls the authenticated user's ID set by SessionAuth.
func UserID(c *gin.Context) (string, bool) {
	v, ok := c.Get(ContextUserIDKey)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}
