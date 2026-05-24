package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/service/agentregistry"
)

// AgentHeartbeat records an implicit heartbeat each time an agent polls.
// If X-Agent-Name is missing the request passes through without recording.
func AgentHeartbeat(svc *agentregistry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.GetHeader("X-Agent-Name")
		if name != "" {
			agentType := c.GetHeader("X-Agent-Type")
			if err := svc.Record(c.Request.Context(), name, c.ClientIP(), agentType); err != nil {
				slog.Error("agent heartbeat failed", "agent", name, "error", err)
			}
		}
		c.Next()
	}
}
