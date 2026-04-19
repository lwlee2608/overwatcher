package intent

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by Store.GetByID when no intent exists with the
// given id.
var ErrNotFound = errors.New("intent not found")

// Store is the interface for managing deploy intents. Production uses DBStore
// (PostgreSQL-backed). MemoryStore exists for tests.
type Store interface {
	Enqueue(i *DeployIntent)
	TakeNext(ctx context.Context) (*DeployIntent, error)
	Complete(id string, success bool) (*DeployIntent, bool)
	Requeue(id string) (*DeployIntent, bool)
	SweepTimedOut(timeout time.Duration, maxAttempts int) (requeued, failed []*DeployIntent)
	List() []*DeployIntent
	ListRecent(ctx context.Context, limit int32) ([]*DeployIntent, error)
	GetByID(ctx context.Context, id string) (*DeployIntent, error)
	InFlight() []*DeployIntent
	Len() int
}
