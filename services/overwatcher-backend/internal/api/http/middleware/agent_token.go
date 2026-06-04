package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/service/agentregistry"
)

const ContextAgentKey = "agent.resolved"

// AgentTokenAuth requires `Authorization: Bearer <agent_token>`, resolves the
// token to an agent, and stashes that agent in the context for downstream
// handlers. A token that matches no agent is a 401 — there is no global secret
// fallback. Identity comes from the token, never from a client-supplied header.
func AgentTokenAuth(svc *agentregistry.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed bearer token"})
			return
		}
		token := header[len(prefix):]
		agent, err := svc.ResolveByToken(c.Request.Context(), token)
		if err != nil {
			if errors.Is(err, agentregistry.ErrNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "token lookup failed"})
			return
		}
		c.Set(ContextAgentKey, agent)
		c.Next()
	}
}

// Agent pulls the agent resolved by AgentTokenAuth from the context.
func Agent(c *gin.Context) (*agentregistry.AgentStatus, bool) {
	v, ok := c.Get(ContextAgentKey)
	if !ok {
		return nil, false
	}
	a, ok := v.(*agentregistry.AgentStatus)
	return a, ok
}
