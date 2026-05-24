package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lwlee2608/overwatcher/internal/api/http/handler"
	"github.com/lwlee2608/overwatcher/internal/protocol"
	"github.com/lwlee2608/overwatcher/internal/service/intent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func authReq(req *http.Request, secret string) {
	req.Header.Set("Authorization", "Bearer "+secret)
}

func webSvc(tag string) []intent.ServiceSpec {
	return []intent.ServiceSpec{{Name: "web", Image: "ghcr.io/owner/repo", Tag: tag}}
}

func TestDeploy(t *testing.T, router *gin.Engine, store *intent.DBStore, bearerToken string) {
	t.Run("NextLongPollTimeoutReturns204", func(t *testing.T) {
		// Runs first while the queue is still empty so no drain is needed.
		prev := handler.LongPollTimeout
		handler.LongPollTimeout = 50 * time.Millisecond
		defer func() { handler.LongPollTimeout = prev }()

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/deploy/next", nil)
		authReq(req, bearerToken)

		start := time.Now()
		router.ServeHTTP(rr, req)
		elapsed := time.Since(start)

		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Less(t, elapsed, 500*time.Millisecond,
			"Next blocked for %v, expected ~50ms timeout", elapsed)
	})

	t.Run("EnqueueAndNext", func(t *testing.T) {
		store.Enqueue(&intent.DeployIntent{
			DeliveryID:     "delivery-1",
			Repo:           "owner/repo",
			Ref:            "refs/heads/main",
			SHA:            "abc123",
			Stack:          "stack-enqueue",
			Services:       webSvc("abc123"),
			Environment:    "production",
			DeploymentID:   100,
			InstallationID: 1,
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/deploy/next", nil)
		authReq(req, bearerToken)
		router.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)

		var resp protocol.DeployIntentResponse
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.ID)
		assert.Equal(t, "delivery-1", resp.DeliveryID)
		assert.Equal(t, "owner/repo", resp.Repo)
		assert.Equal(t, "refs/heads/main", resp.Ref)
		assert.Equal(t, "abc123", resp.SHA)
		assert.Equal(t, "stack-enqueue", resp.Stack)
		require.Len(t, resp.Services, 1)
		assert.Equal(t, "web", resp.Services[0].Name)
		assert.Equal(t, "ghcr.io/owner/repo", resp.Services[0].Image)
		assert.Equal(t, "abc123", resp.Services[0].Tag)
		assert.Equal(t, "production", resp.Environment)
		assert.Equal(t, int64(100), resp.DeploymentID)

		// Clean up: complete the intent so it doesn't block other tests.
		store.Complete(resp.ID, true)
	})

	t.Run("ReportSuccess", func(t *testing.T) {
		store.Enqueue(&intent.DeployIntent{
			DeliveryID:     "delivery-success",
			Repo:           "owner/repo",
			Ref:            "refs/heads/main",
			SHA:            "def456",
			Stack:          "stack-success",
			Services:       webSvc("def456"),
			Environment:    "staging",
			DeploymentID:   101,
			InstallationID: 1,
		})

		// Take the intent via HTTP.
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/deploy/next", nil)
		authReq(req, bearerToken)
		router.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)

		var resp protocol.DeployIntentResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

		// Report success via HTTP.
		body, _ := json.Marshal(protocol.DeployResultRequest{State: "success"})
		rr = httptest.NewRecorder()
		req = httptest.NewRequest("POST", "/api/v1/deploy/"+resp.ID+"/result", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		authReq(req, bearerToken)
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Empty(t, store.InFlight())
	})

	t.Run("ReportFailure", func(t *testing.T) {
		store.Enqueue(&intent.DeployIntent{
			DeliveryID:     "delivery-failure",
			Repo:           "owner/repo",
			Ref:            "refs/heads/main",
			SHA:            "ghi789",
			Stack:          "stack-failure",
			Services:       webSvc("ghi789"),
			Environment:    "staging",
			DeploymentID:   102,
			InstallationID: 1,
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/v1/deploy/next", nil)
		authReq(req, bearerToken)
		router.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)

		var resp protocol.DeployIntentResponse
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

		body, _ := json.Marshal(protocol.DeployResultRequest{State: "failure", Error: "compose pull exit 1"})
		rr = httptest.NewRecorder()
		req = httptest.NewRequest("POST", "/api/v1/deploy/"+resp.ID+"/result", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		authReq(req, bearerToken)
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Empty(t, store.InFlight())
	})

	t.Run("ReportUnknownID", func(t *testing.T) {
		body, _ := json.Marshal(protocol.DeployResultRequest{State: "success"})
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/deploy/00000000-0000-0000-0000-000000000000/result", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		authReq(req, bearerToken)
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("ConcurrencyGuardSameStack", func(t *testing.T) {
		store.Enqueue(&intent.DeployIntent{
			DeliveryID:     "delivery-guard-1",
			Repo:           "owner/repo",
			Ref:            "refs/heads/main",
			SHA:            "aaa111",
			Stack:          "stack-guard",
			Services:       webSvc("aaa111"),
			Environment:    "production",
			DeploymentID:   200,
			InstallationID: 1,
		})
		store.Enqueue(&intent.DeployIntent{
			DeliveryID:     "delivery-guard-2",
			Repo:           "owner/repo",
			Ref:            "refs/heads/main",
			SHA:            "bbb222",
			Stack:          "stack-guard",
			Services:       webSvc("bbb222"),
			Environment:    "production",
			DeploymentID:   201,
			InstallationID: 1,
		})

		// First take succeeds.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		first, err := store.TakeNext(ctx)
		require.NoError(t, err)
		assert.Equal(t, "aaa111", first.SHA)

		// Second take should block — same stack is in-flight.
		ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel2()
		_, err = store.TakeNext(ctx2)
		assert.ErrorIs(t, err, context.DeadlineExceeded)

		// Complete first — second should now be available.
		store.Complete(first.ID, true)
		ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel3()
		second, err := store.TakeNext(ctx3)
		require.NoError(t, err)
		assert.Equal(t, "bbb222", second.SHA)

		store.Complete(second.ID, true)
	})

	t.Run("WebhookDedup", func(t *testing.T) {
		// Dedup is keyed on (delivery_id, project_id) via a partial unique
		// index (WHERE project_id IS NOT NULL), so both rows must carry the
		// same project_id for the conflict to fire.
		dupProjectID := "11111111-1111-1111-1111-111111111111"
		store.Enqueue(&intent.DeployIntent{
			DeliveryID:     "delivery-dup",
			ProjectID:      dupProjectID,
			Repo:           "owner/repo",
			Ref:            "refs/heads/main",
			SHA:            "ccc333",
			Stack:          "stack-dup",
			Services:       webSvc("ccc333"),
			Environment:    "production",
			DeploymentID:   300,
			InstallationID: 1,
		})
		store.Enqueue(&intent.DeployIntent{
			DeliveryID:     "delivery-dup",
			ProjectID:      dupProjectID,
			Repo:           "owner/repo",
			Ref:            "refs/heads/main",
			SHA:            "ccc333",
			Stack:          "stack-dup",
			Services:       webSvc("ccc333"),
			Environment:    "production",
			DeploymentID:   300,
			InstallationID: 1,
		})

		// Only one intent should exist for this stack.
		var count int
		for _, i := range store.List() {
			if i.Stack == "stack-dup" {
				count++
			}
		}
		assert.Equal(t, 1, count, "duplicate webhook delivery should be deduped")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		i, err := store.TakeNext(ctx)
		require.NoError(t, err)
		store.Complete(i.ID, true)
	})

	t.Run("SweepTimedOutRequeue", func(t *testing.T) {
		store.Enqueue(&intent.DeployIntent{
			DeliveryID:     "delivery-sweep-rq",
			Repo:           "owner/repo",
			Ref:            "refs/heads/main",
			SHA:            "ddd444",
			Stack:          "stack-sweep-rq",
			Services:       webSvc("ddd444"),
			Environment:    "production",
			DeploymentID:   400,
			InstallationID: 1,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		taken, err := store.TakeNext(ctx)
		require.NoError(t, err)

		// Let dispatched_at age past the sweep cutoff.
		time.Sleep(15 * time.Millisecond)
		requeued, failed := store.SweepTimedOut(time.Millisecond, 3)
		assert.Len(t, requeued, 1)
		assert.Empty(t, failed)
		assert.Equal(t, taken.ID, requeued[0].ID)

		// Intent is back in the queue — clean up.
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		i, err := store.TakeNext(ctx2)
		require.NoError(t, err)
		store.Complete(i.ID, true)
	})

	t.Run("SweepTimedOutPermanentFailure", func(t *testing.T) {
		store.Enqueue(&intent.DeployIntent{
			DeliveryID:     "delivery-sweep-fail",
			Repo:           "owner/repo",
			Ref:            "refs/heads/main",
			SHA:            "eee555",
			Stack:          "stack-sweep-fail",
			Services:       webSvc("eee555"),
			Environment:    "production",
			DeploymentID:   500,
			InstallationID: 1,
		})

		// TakeNext sets attempts=1.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := store.TakeNext(ctx)
		require.NoError(t, err)

		// Sweep with maxAttempts=1 → permanently failed.
		time.Sleep(15 * time.Millisecond)
		requeued, failed := store.SweepTimedOut(time.Millisecond, 1)
		assert.Empty(t, requeued)
		assert.Len(t, failed, 1)
		assert.Equal(t, intent.StatusPermanentlyFailed, failed[0].Status)
	})
}
