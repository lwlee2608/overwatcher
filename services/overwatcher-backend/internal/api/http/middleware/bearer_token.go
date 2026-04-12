package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// BearerTokenAuth requires `Authorization: Bearer <secret>` and aborts with
// 401 on any mismatch. Comparison is constant-time to avoid token-length leaks.
func BearerTokenAuth(secret string) gin.HandlerFunc {
	expected := []byte(secret)
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed bearer token"})
			return
		}
		got := []byte(header[len(prefix):])
		if subtle.ConstantTimeCompare(got, expected) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid bearer token"})
			return
		}
		c.Next()
	}
}
