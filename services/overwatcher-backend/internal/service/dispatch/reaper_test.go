package dispatch

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lwlee2608/overwatcher/internal/service/intent"
)

// fakeUpdater records calls to UpdateDeploymentStatus.
type fakeUpdater struct {
	mu    sync.Mutex
	calls []fakeCall
}

type fakeCall struct {
	InstallationID int64
	Owner          string
	Repo           string
	DeploymentID   int64
	State          string
	Description    string
}

func (f *fakeUpdater) UpdateDeploymentStatus(_ context.Context, installationID int64, owner, repo string, deploymentID int64, state, description string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCall{installationID, owner, repo, deploymentID, state, description})
	return nil
}

func (f *fakeUpdater) getCalls() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]fakeCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func TestReaper_SweepsAndUpdatesGitHub(t *testing.T) {
	store := intent.NewStore()
	updater := &fakeUpdater{}

	store.Enqueue(&intent.DeployIntent{
		ID:             "a",
		Stack:          "s1",
		Repo:           "owner/repo",
		InstallationID: 42,
		DeploymentID:   99,
	})
	i, _ := store.TakeNext(context.Background())

	// Backdate and set attempts to trigger permanent failure.
	store.TestSetInFlight(i.ID, func(di *intent.DeployIntent) {
		di.DispatchedAt = time.Now().Add(-15 * time.Minute)
		di.Attempts = 3
	})

	reaper := NewReaper(store, updater, 10*time.Minute, 3, 50*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	reaper.Run(ctx)

	calls := updater.getCalls()
	if len(calls) == 0 {
		t.Fatal("expected at least one GitHub update call")
	}
	c := calls[0]
	if c.State != "failure" {
		t.Errorf("state = %q, want failure", c.State)
	}
	if c.Owner != "owner" || c.Repo != "repo" {
		t.Errorf("owner/repo = %s/%s, want owner/repo", c.Owner, c.Repo)
	}
	if c.DeploymentID != 99 {
		t.Errorf("deploymentID = %d, want 99", c.DeploymentID)
	}
}

func TestReaper_StopsOnContextCancel(t *testing.T) {
	store := intent.NewStore()
	updater := &fakeUpdater{}
	reaper := NewReaper(store, updater, time.Minute, 3, time.Hour)

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
}
