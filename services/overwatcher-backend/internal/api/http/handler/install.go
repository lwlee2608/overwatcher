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
		scheme := "https"
		// Trust X-Forwarded-Proto when behind a proxy; otherwise gin's TLS field.
		if c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") == "" {
			scheme = "http"
		} else if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			scheme = proto
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
