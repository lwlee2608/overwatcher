package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/service/agentregistry"
)

// AgentHeartbeat records an implicit heartbeat on the agent resolved by
// AgentTokenAuth. Identity is the token, not a header; X-Agent-Type/Version are
// still trusted as descriptive metadata only.
func AgentHeartbeat(svc *agentregistry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if a, ok := Agent(c); ok {
			agentType := c.GetHeader("X-Agent-Type")
			version := c.GetHeader("X-Agent-Version")
			if err := svc.Touch(c.Request.Context(), a.ID, c.ClientIP(), agentType, version); err != nil {
				slog.Error("agent heartbeat failed", "agent", a.Name, "error", err)
			}
		}
		c.Next()
	}
}
