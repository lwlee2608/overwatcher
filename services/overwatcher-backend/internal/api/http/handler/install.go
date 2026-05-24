package handler

import (
	_ "embed"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed install-agent.sh
var installScript string

type InstallHandler struct {
	releaseTag string
	publicURL  string
}

func NewInstallHandler(releaseTag, publicURL string) *InstallHandler {
	if releaseTag == "" {
		releaseTag = "latest"
	}
	return &InstallHandler{releaseTag: releaseTag, publicURL: publicURL}
}

func (h *InstallHandler) Serve(c *gin.Context) {
	coordinator := h.publicURL
	if coordinator == "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		} else if c.GetHeader("X-Forwarded-For") != "" || c.GetHeader("X-Forwarded-Host") != "" {
			// Behind a proxy that didn't set X-Forwarded-Proto (Railway's edge
			// is one). Real-world proxies terminate TLS upstream, so plain
			// http here would silently produce a broken AGENT_COORDINATOR_URL
			// — and the agent now refuses to start on http to non-loopback.
			scheme = "https"
		}
		coordinator = scheme + "://" + c.Request.Host
	}

	body := strings.NewReplacer(
		"{{COORDINATOR_URL}}", coordinator,
		"{{RELEASE_TAG}}", h.releaseTag,
	).Replace(installScript)

	c.Header("Content-Type", "text/x-shellscript; charset=utf-8")
	c.String(http.StatusOK, body)
}
