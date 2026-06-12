package middleware

import (
	"encoding/json"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/protocol"
	"github.com/lwlee2608/overwatcher/internal/service/agentregistry"
)

// AgentHeartbeat records an implicit heartbeat on the agent resolved by
// AgentTokenAuth. Identity is the token, not a header; X-Agent-Type/Version/
// Metrics are still trusted as descriptive metadata only.
func AgentHeartbeat(svc *agentregistry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if a, ok := Agent(c); ok {
			agentType := c.GetHeader("X-Agent-Type")
			version := c.GetHeader("X-Agent-Version")
			metrics := parseHostMetrics(c.GetHeader(protocol.HostMetricsHeader))
			if err := svc.Touch(c.Request.Context(), a.ID, c.ClientIP(), agentType, version, metrics); err != nil {
				slog.Error("agent heartbeat failed", "agent", a.Name, "error", err)
			}
		}
		c.Next()
	}
}

// parseHostMetrics returns nil on an absent or malformed header — metrics are
// best-effort and must never fail the heartbeat.
func parseHostMetrics(raw string) *agentregistry.HostMetrics {
	if raw == "" {
		return nil
	}
	var m protocol.HostMetrics
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return &agentregistry.HostMetrics{
		CPUPercent:     m.CPUPercent,
		MemUsedBytes:   m.MemUsedBytes,
		MemTotalBytes:  m.MemTotalBytes,
		SwapUsedBytes:  m.SwapUsedBytes,
		SwapTotalBytes: m.SwapTotalBytes,
		DiskUsedBytes:  m.DiskUsedBytes,
		DiskTotalBytes: m.DiskTotalBytes,
	}
}
