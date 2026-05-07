package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lwlee2608/overwatcher/internal/service/intent"
	"github.com/lwlee2608/overwatcher/internal/service/webhook"
)

// TestWebhookRedeploy covers webhook.Service.Redeploy precondition checks
// against a real DBStore — the early-exit branches that don't require a
// GitHub installation client. The happy path needs a real GitHub App and
// isn't covered here.
func TestWebhookRedeploy(t *testing.T, pool *pgxpool.Pool) {
	t.Run("SourceNotFound", func(t *testing.T) {
		store := freshIntentStore(t, pool)
		svc := webhook.New(nil, nil, store, nil)

		err := svc.Redeploy(context.Background(), "00000000-0000-0000-0000-000000000000")
		if !errors.Is(err, intent.ErrNotFound) {
			t.Fatalf("expected intent.ErrNotFound, got %v", err)
		}
	})

	t.Run("MissingInstallationID", func(t *testing.T) {
		store := freshIntentStore(t, pool)
		store.Enqueue(&intent.DeployIntent{
			DeliveryID:     "src-1",
			Repo:           "owner/repo",
			Ref:            "refs/heads/main",
			SHA:            "abcdef1234567890",
			Stack:          "prod-agent",
			Services:       []intent.ServiceSpec{{Name: "web", Image: "x", Tag: "v1"}},
			Environment:    "production",
			ComposeFile:    "docker-compose.yml",
			DeploymentID:   1,
			InstallationID: 0, // the bit under test
		})

		// Look up the persisted ID — DBStore generates it server-side.
		src := store.List()[0]

		svc := webhook.New(nil, nil, store, nil)
		err := svc.Redeploy(context.Background(), src.ID)
		if !errors.Is(err, webhook.ErrNoInstallation) {
			t.Fatalf("expected ErrNoInstallation, got %v", err)
		}
	})

	t.Run("MalformedRepo", func(t *testing.T) {
		store := freshIntentStore(t, pool)
		store.Enqueue(&intent.DeployIntent{
			DeliveryID:     "src-2",
			Repo:           "no-slash-here",
			Ref:            "refs/heads/main",
			SHA:            "abcdef",
			Stack:          "prod-agent",
			Environment:    "production",
			ComposeFile:    "docker-compose.yml",
			DeploymentID:   1,
			InstallationID: 42,
		})
		src := store.List()[0]

		svc := webhook.New(nil, nil, store, nil)
		err := svc.Redeploy(context.Background(), src.ID)
		if !errors.Is(err, webhook.ErrInvalidRepo) {
			t.Fatalf("expected ErrInvalidRepo, got %v", err)
		}
	})
}
