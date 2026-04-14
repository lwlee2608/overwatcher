package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthCheck(t *testing.T, router *gin.Engine) {
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var resp dto.HealthResponse
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
}
