package service

import (
	"sync"
	"time"
)

// IntentStatus tracks a DeployIntent through its lifecycle. Phase 2 only writes
// IntentCreated; later phases progress intents through dispatched -> succeeded/failed.
type IntentStatus string

const (
	IntentCreated    IntentStatus = "created"
	IntentDispatched IntentStatus = "dispatched"
	IntentSucceeded  IntentStatus = "succeeded"
	IntentFailed     IntentStatus = "failed"
)

// DeployIntent is everything an agent needs to perform a deploy. It is produced
// on a push webhook when the repo matches a MappingEntry.
type DeployIntent struct {
	ID           string
	CreatedAt    time.Time
	DeliveryID   string
	Repo         string
	Ref          string
	SHA          string
	Image        string
	Tag          string
	Stack        string
	Services     []string
	Environment  string
	DeploymentID int64
	Status       IntentStatus
}

// IntentStore is an in-memory, thread-safe queue of DeployIntents. Phase 2 only
// enqueues; Phase 3 will add dispatching.
type IntentStore struct {
	mu      sync.Mutex
	intents []*DeployIntent
}

func NewIntentStore() *IntentStore {
	return &IntentStore{}
}

func (s *IntentStore) Enqueue(intent *DeployIntent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.intents = append(s.intents, intent)
}

// List returns a shallow copy of the current queue for inspection.
func (s *IntentStore) List() []*DeployIntent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*DeployIntent, len(s.intents))
	copy(out, s.intents)
	return out
}

func (s *IntentStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.intents)
}
