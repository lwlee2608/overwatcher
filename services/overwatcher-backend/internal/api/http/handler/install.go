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
	releaseTag   string
	publicURL    string
	sharedSecret string
}

func NewInstallHandler(releaseTag, publicURL, sharedSecret string) *InstallHandler {
	if releaseTag == "" {
		releaseTag = "latest"
	}
	return &InstallHandler{releaseTag: releaseTag, publicURL: publicURL, sharedSecret: sharedSecret}
}

// Config returns the values the dashboard needs to render a ready-to-paste
// install command. Session-protected — the shared secret authenticates every
// agent to the coordinator, so only logged-in users may read it.
func (h *InstallHandler) Config(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"shared_secret": h.sharedSecret,
	})
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
			// Behind a proxy (Railway, Cloudflare) that omitted Proto —
			// assume TLS terminated upstream.
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
