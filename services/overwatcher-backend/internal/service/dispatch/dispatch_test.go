package dispatch

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/lwlee2608/overwatcher/internal/service/intent"
)

type fakeStatusUpdater struct {
	mu    sync.Mutex
	calls []statusCall
	err   error
}

type statusCall struct {
	installationID int64
	owner          string
	repo           string
	deploymentID   int64
	state          string
	description    string
}

func (f *fakeStatusUpdater) UpdateDeploymentStatus(ctx context.Context, installationID int64, owner, repo string, deploymentID int64, state, description string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, statusCall{installationID, owner, repo, deploymentID, state, description})
	return f.err
}

func (f *fakeStatusUpdater) snapshot() []statusCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]statusCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func newTestDispatch() (*Service, *intent.MemoryStore, *fakeStatusUpdater) {
	store := intent.NewMemoryStore()
	upd := &fakeStatusUpdater{}
	return &Service{store: store, updater: upd}, store, upd
}

func TestService_Next_HappyPath(t *testing.T) {
	d, store, upd := newTestDispatch()

	store.Enqueue(&intent.DeployIntent{
		ID:             "i1",
		Repo:           "owner/repo",
		Stack:          "foo",
		DeploymentID:   42,
		InstallationID: 7,
		Status:         intent.StatusCreated,
	})

	got, err := d.Next(context.Background())
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got.ID != "i1" {
		t.Errorf("got id %q, want i1", got.ID)
	}
	if got.Status != intent.StatusDispatched {
		t.Errorf("status = %q, want StatusDispatched", got.Status)
	}

	calls := upd.snapshot()
	if len(calls) != 1 {
		t.Fatalf("got %d updater calls, want 1", len(calls))
	}
	c := calls[0]
	if c.state != "in_progress" {
		t.Errorf("state = %q, want in_progress", c.state)
	}
	if c.owner != "owner" || c.repo != "repo" {
		t.Errorf("owner/repo = %q/%q, want owner/repo", c.owner, c.repo)
	}
	if c.installationID != 7 || c.deploymentID != 42 {
		t.Errorf("installation/deployment IDs wrong: %+v", c)
	}
}

func TestService_Next_StatusFailureStillReturnsIntent(t *testing.T) {
	d, store, upd := newTestDispatch()
	upd.err = errors.New("github 503")

	store.Enqueue(&intent.DeployIntent{ID: "i1", Repo: "o/r", InstallationID: 1})

	got, err := d.Next(context.Background())
	if err != nil {
		t.Fatalf("Next must not propagate updater errors: %v", err)
	}
	if got == nil || got.ID != "i1" {
		t.Errorf("intent not returned: %+v", got)
	}
	// Intent must still be in-flight even though the GitHub call failed.
	if len(store.InFlight()) != 1 {
		t.Errorf("InFlight len = %d, want 1", len(store.InFlight()))
	}
}

func TestService_Next_CtxCancelled(t *testing.T) {
	d, _, _ := newTestDispatch()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := d.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestService_Report_HappyPath(t *testing.T) {
	d, store, upd := newTestDispatch()
	store.Enqueue(&intent.DeployIntent{ID: "i1", Repo: "o/r", DeploymentID: 99, InstallationID: 3})
	if _, err := d.Next(context.Background()); err != nil {
		t.Fatalf("Next: %v", err)
	}

	if !d.Report(context.Background(), "i1", true, "") {
		t.Fatal("Report returned false for known id")
	}

	if got := len(store.InFlight()); got != 0 {
		t.Errorf("InFlight len = %d, want 0", got)
	}

	calls := upd.snapshot()
	if len(calls) != 2 {
		t.Fatalf("got %d updater calls, want 2", len(calls))
	}
	if calls[1].state != "success" {
		t.Errorf("state = %q, want success", calls[1].state)
	}
	if calls[1].deploymentID != 99 {
		t.Errorf("deploymentID = %d, want 99", calls[1].deploymentID)
	}
}

func TestService_Report_FailureCarriesErrorMsg(t *testing.T) {
	d, store, upd := newTestDispatch()
	store.Enqueue(&intent.DeployIntent{ID: "i1", Repo: "o/r"})
	_, _ = d.Next(context.Background())

	if !d.Report(context.Background(), "i1", false, "compose pull exit 1") {
		t.Fatal("Report returned false")
	}

	calls := upd.snapshot()
	last := calls[len(calls)-1]
	if last.state != "failure" {
		t.Errorf("state = %q, want failure", last.state)
	}
	if last.description != "Deploy failed: compose pull exit 1" {
		t.Errorf("description = %q", last.description)
	}
}

func TestService_Report_UnknownID(t *testing.T) {
	d, _, _ := newTestDispatch()

	if d.Report(context.Background(), "ghost", true, "") {
		t.Error("Report returned true for unknown id")
	}
}
