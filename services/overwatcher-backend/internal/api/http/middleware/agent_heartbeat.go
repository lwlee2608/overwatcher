package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/service/agent"
)

// AgentHeartbeat records an implicit heartbeat each time an agent polls.
// If X-Agent-Name is missing the request passes through without recording.
func AgentHeartbeat(tracker *agent.Tracker) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.GetHeader("X-Agent-Name")
		if name != "" {
			var stacks []string
			if raw := c.GetHeader("X-Agent-Stacks"); raw != "" {
				for _, s := range strings.Split(raw, ",") {
					if trimmed := strings.TrimSpace(s); trimmed != "" {
						stacks = append(stacks, trimmed)
					}
				}
			}
			tracker.Record(name, stacks, c.ClientIP())
		}
		c.Next()
	}
}
