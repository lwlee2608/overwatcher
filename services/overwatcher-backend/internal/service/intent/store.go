package intent

import (
	"context"
	"time"
)

// Store is the interface for managing deploy intents. Production uses DBStore
// (PostgreSQL-backed). MemoryStore exists for tests.
type Store interface {
	Enqueue(i *DeployIntent)
	TakeNext(ctx context.Context) (*DeployIntent, error)
	Complete(id string, success bool) (*DeployIntent, bool)
	Requeue(id string) (*DeployIntent, bool)
	SweepTimedOut(timeout time.Duration, maxAttempts int) (requeued, failed []*DeployIntent)
	List() []*DeployIntent
	InFlight() []*DeployIntent
	Len() int
}
