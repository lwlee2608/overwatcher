package handler

import (
	"net/http"

	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/gin-gonic/gin"
)

// VersionHandler reports the coordinator's build version and the agent release
// tag derived from it (the release the install command installs).
type VersionHandler struct {
	version    string
	releaseTag string
}

func NewVersionHandler(version, releaseTag string) *VersionHandler {
	return &VersionHandler{version: version, releaseTag: releaseTag}
}

func (h *VersionHandler) Get(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, dto.VersionResponse{
		Version:    h.version,
		ReleaseTag: h.releaseTag,
	})
}
