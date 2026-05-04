package intent

import (
	"context"
	"testing"
	"time"
)

// newTestStore truncates the intents table and returns a fresh DBStore
// scoped to a single test.
func newTestStore(t *testing.T) *DBStore {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := testPool.Exec(ctx, "TRUNCATE deploy_intents"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return NewDBStore(testPool)
}

// newIntent builds a minimally-valid DeployIntent with all NOT NULL columns
// filled. deliveryID must be unique within a test.
func newIntent(deliveryID, stack string) *DeployIntent {
	return &DeployIntent{
		DeliveryID:     deliveryID,
		Repo:           "owner/repo",
		Ref:            "refs/heads/main",
		SHA:            "deadbeef",
		Stack:          stack,
		Environment:    "test",
		ComposeFile:    "docker-compose.yml",
		DeploymentID:   1,
		InstallationID: 1,
	}
}

// backdateInFlight overrides dispatched_at and attempts on an in-flight
// intent via direct SQL — replaces MemoryStore's TestSetInFlight.
func backdateInFlight(t *testing.T, id string, dispatchedAt time.Time, attempts int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := testPool.Exec(ctx,
		`UPDATE deploy_intents SET dispatched_at = $1, attempts = $2 WHERE id = $3`,
		dispatchedAt.UTC(), attempts, parseUUID(id))
	if err != nil {
		t.Fatalf("backdate: %v", err)
	}
}
