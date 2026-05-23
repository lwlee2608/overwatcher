package handler

import (
	_ "embed"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// installScript is the systemd-agent installer served at GET /install.sh.
// The {{COORDINATOR_URL}} and {{RELEASE_TAG}} markers are substituted at
// request time from the coordinator's config.
//
//go:embed install-agent.sh
var installScript string

type InstallHandler struct {
	releaseTag string
	publicURL  string
}

// NewInstallHandler builds the handler. releaseTag is the GitHub release the
// installer downloads from ("latest" or a pinned tag). publicURL is the URL
// agents should poll back to; an empty value falls back to the request's
// own Host header at serve time.
func NewInstallHandler(releaseTag, publicURL string) *InstallHandler {
	if releaseTag == "" {
		releaseTag = "latest"
	}
	return &InstallHandler{releaseTag: releaseTag, publicURL: publicURL}
}

// Serve renders the installer with coordinator URL and release tag baked in.
// Public endpoint — no auth — because the user pipes it into bash before any
// credentials exist on the VM.
func (h *InstallHandler) Serve(c *gin.Context) {
	coordinator := h.publicURL
	if coordinator == "" {
		scheme := "https"
		// gin sets c.Request.TLS for direct TLS termination; behind a proxy
		// we trust X-Forwarded-Proto when set.
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
