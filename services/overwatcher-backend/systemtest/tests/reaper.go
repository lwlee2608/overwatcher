package tests

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lwlee2608/overwatcher/internal/service/dispatch"
	"github.com/lwlee2608/overwatcher/internal/service/intent"
)

// TestReaper exercises dispatch.Reaper against a real DBStore. backdateInFlight
// (defined in intent.go) writes dispatched_at/attempts directly so we can
// simulate stale dispatches without sleeping.
func TestReaper(t *testing.T, pool *pgxpool.Pool) {
	t.Run("SweepsAndUpdatesGitHub", func(t *testing.T) {
		store := freshIntentStore(t, pool)
		updater := &fakeStatusUpdater{}

		store.Enqueue(&intent.DeployIntent{
			DeliveryID:     "d1",
			Repo:           "owner/repo",
			Ref:            "refs/heads/main",
			SHA:            "deadbeef",
			Stack:          "s1",
			Environment:    "test",
			ComposeFile:    "docker-compose.yml",
			DeploymentID:   99,
			InstallationID: 42,
		})
		i, err := store.TakeNext(context.Background())
		if err != nil {
			t.Fatalf("TakeNext: %v", err)
		}

		// Backdate to before the timeout cutoff and bump attempts past the
		// max so the sweep classifies it as permanently failed.
		backdateInFlight(t, pool, i.ID, time.Now().Add(-15*time.Minute), 3)

		reaper := dispatch.NewReaper(store, updater, 10*time.Minute, 3, 50*time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		reaper.Run(ctx)

		calls := updater.snapshot()
		if len(calls) == 0 {
			t.Fatal("expected at least one GitHub update call")
		}
		c := calls[0]
		if c.state != "failure" {
			t.Errorf("state = %q, want failure", c.state)
		}
		if c.owner != "owner" || c.repo != "repo" {
			t.Errorf("owner/repo = %s/%s, want owner/repo", c.owner, c.repo)
		}
		if c.deploymentID != 99 {
			t.Errorf("deploymentID = %d, want 99", c.deploymentID)
		}
	})

	t.Run("StopsOnContextCancel", func(t *testing.T) {
		store := freshIntentStore(t, pool)
		updater := &fakeStatusUpdater{}
		reaper := dispatch.NewReaper(store, updater, time.Minute, 3, time.Hour)

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			reaper.Run(ctx)
			close(done)
		}()

		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Reaper did not exit after context cancel")
		}
	})
}
