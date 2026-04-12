package intent

import (
	"context"
	"time"
)

// Store is the interface for managing deploy intents. It is implemented by
// MemoryStore (in-process, for tests and simple deployments) and DBStore
// (PostgreSQL-backed, for persistence across restarts).
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
