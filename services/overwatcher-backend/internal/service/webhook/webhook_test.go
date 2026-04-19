package webhook

import (
	"context"
	"errors"
	"testing"

	"github.com/lwlee2608/overwatcher/internal/service/intent"
	"github.com/lwlee2608/overwatcher/internal/service/mapping"
)

func TestRedeploy_SourceNotFound(t *testing.T) {
	store := intent.NewMemoryStore()
	svc := New(nil, nil, store, nil)

	err := svc.Redeploy(context.Background(), "00000000-0000-0000-0000-000000000000")
	if !errors.Is(err, intent.ErrNotFound) {
		t.Fatalf("expected intent.ErrNotFound, got %v", err)
	}
}

func TestRedeploy_MissingInstallationID(t *testing.T) {
	store := intent.NewMemoryStore()
	src := &intent.DeployIntent{
		ID:             "src-1",
		DeliveryID:     "orig-delivery",
		Repo:           "owner/repo",
		Ref:            "refs/heads/main",
		SHA:            "abcdef1234567890",
		Stack:          "prod-agent",
		Services:       []mapping.ServiceSpec{{Name: "web", Image: "x", Tag: "v1"}},
		Environment:    "production",
		InstallationID: 0,
		Status:         intent.StatusCreated,
	}
	store.Enqueue(src)

	svc := New(nil, nil, store, nil)
	err := svc.Redeploy(context.Background(), "src-1")
	if !errors.Is(err, ErrNoInstallation) {
		t.Fatalf("expected ErrNoInstallation, got %v", err)
	}
}

func TestRedeploy_MalformedRepo(t *testing.T) {
	store := intent.NewMemoryStore()
	src := &intent.DeployIntent{
		ID:             "src-2",
		DeliveryID:     "orig-delivery",
		Repo:           "no-slash-here",
		SHA:            "abcdef",
		Stack:          "prod-agent",
		Environment:    "production",
		InstallationID: 42,
		Status:         intent.StatusCreated,
	}
	store.Enqueue(src)

	svc := New(nil, nil, store, nil)
	err := svc.Redeploy(context.Background(), "src-2")
	if !errors.Is(err, ErrInvalidRepo) {
		t.Fatalf("expected ErrInvalidRepo, got %v", err)
	}
}

