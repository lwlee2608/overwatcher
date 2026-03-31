package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func WebhookSignatureVerifier(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Set("webhookPayload", body)

		signature := c.GetHeader("X-Hub-Signature-256")
		if signature == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing signature"})
			return
		}

		sig := strings.TrimPrefix(signature, "sha256=")
		decoded, err := hex.DecodeString(sig)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid signature format"})
			return
		}

		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expected := mac.Sum(nil)

		if !hmac.Equal(decoded, expected) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "signature mismatch"})
			return
		}

		c.Next()
	}
}
