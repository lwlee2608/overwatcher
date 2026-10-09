package handler

import (
	_ "embed"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed install-cli.sh
var cliScript string

type CLIHandler struct {
	releaseTag string
}

func NewCLIHandler(releaseTag string) *CLIHandler {
	if releaseTag == "" {
		releaseTag = "latest"
	}
	return &CLIHandler{releaseTag: releaseTag}
}

func (h *CLIHandler) Serve(c *gin.Context) {
	tag := strings.ReplaceAll(h.releaseTag, "'", "'\"'\"'")
	body := strings.ReplaceAll(cliScript, "{{RELEASE_TAG}}", tag)
	c.Header("Content-Type", "text/x-shellscript; charset=utf-8")
	c.String(http.StatusOK, body)
}
