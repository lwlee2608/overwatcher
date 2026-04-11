package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/service/dispatch"
	"github.com/lwlee2608/overwatcher/internal/service/intent"
)

func newDeployTestRouter(t *testing.T) (*gin.Engine, *intent.Store) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	store := intent.NewStore()
	svc := dispatch.NewForTest(store)
	h := NewDeployHandler(svc)

	r := gin.New()
	r.GET("/api/v1/deploy/next", h.Next)
	return r, store
}

func TestDeployHandler_Next_LongPollTimeoutReturns204(t *testing.T) {
	prev := longPollTimeout
	longPollTimeout = 50 * time.Millisecond
	defer func() { longPollTimeout = prev }()

	r, _ := newDeployTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/deploy/next", nil)
	w := httptest.NewRecorder()

	start := time.Now()
	r.ServeHTTP(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("Next blocked for %v, expected ~50ms timeout", elapsed)
	}
}

func TestDeployHandler_Next_ReturnsIntentWhenAvailable(t *testing.T) {
	r, store := newDeployTestRouter(t)
	store.Enqueue(&intent.DeployIntent{ID: "i1", Repo: "o/r", Stack: "foo", DeploymentID: 1})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/deploy/next", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !contains(w.Body.String(), `"id":"i1"`) {
		t.Errorf("body did not include intent id: %s", w.Body.String())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
