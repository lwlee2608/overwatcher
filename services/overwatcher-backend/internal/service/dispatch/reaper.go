package dispatch

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lwlee2608/overwatcher/internal/service/intent"
)

// Reaper periodically sweeps in-flight intents that have exceeded the dispatch
// timeout. Timed-out intents are either requeued (under maxAttempts) or
// permanently failed (at or above maxAttempts), with GitHub updated accordingly.
type Reaper struct {
	store       *intent.Store
	updater     StatusUpdater
	timeout     time.Duration
	maxAttempts int
	interval    time.Duration
}

// NewReaper constructs a Reaper. Call Run in a goroutine.
func NewReaper(store *intent.Store, updater StatusUpdater, timeout time.Duration, maxAttempts int, interval time.Duration) *Reaper {
	return &Reaper{
		store:       store,
		updater:     updater,
		timeout:     timeout,
		maxAttempts: maxAttempts,
		interval:    interval,
	}
}

// Run sweeps on every tick until ctx is cancelled.
func (r *Reaper) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.sweep(ctx)
		}
	}
}

func (r *Reaper) sweep(ctx context.Context) {
	requeued, failed := r.store.SweepTimedOut(r.timeout, r.maxAttempts)

	for _, i := range requeued {
		slog.Warn("Intent timed out, requeued",
			"intent_id", i.ID,
			"attempts", i.Attempts,
			"stack", i.Stack,
		)
	}

	for _, i := range failed {
		slog.Error("Intent permanently failed after max attempts",
			"intent_id", i.ID,
			"attempts", i.Attempts,
			"stack", i.Stack,
		)
		owner, repo := splitRepo(i.Repo)
		desc := fmt.Sprintf("Deploy failed: timed out after %d attempts", i.Attempts)
		if err := r.updater.UpdateDeploymentStatus(ctx, i.InstallationID, owner, repo, i.DeploymentID, "failure", desc); err != nil {
			slog.Warn("Failed to update GitHub for permanently failed intent",
				"intent_id", i.ID,
				"error", err,
			)
		}
	}
}
