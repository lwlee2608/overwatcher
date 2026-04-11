package intent

import (
	"context"
	"sync"
	"time"
)

// Status tracks a DeployIntent through its lifecycle.
type Status string

const (
	StatusCreated    Status = "created"
	StatusDispatched Status = "dispatched"
	StatusSucceeded  Status = "succeeded"
	StatusFailed     Status = "failed"
)

// DeployIntent is everything an agent needs to perform a deploy. It is produced
// on a push webhook when the repo matches a mapping.Entry.
type DeployIntent struct {
	ID             string
	CreatedAt      time.Time
	DeliveryID     string
	Repo           string
	Ref            string
	SHA            string
	Image          string
	Tag            string
	Stack          string
	Services       []string
	Environment    string
	DeploymentID   int64
	InstallationID int64
	Status         Status
}

// Store is an in-memory, thread-safe queue of DeployIntents plus an in-flight
// map for intents currently being executed by an agent. TakeNext blocks until
// an intent is enqueued or the caller's context is cancelled.
type Store struct {
	mu       sync.Mutex
	queue    []*DeployIntent
	inFlight map[string]*DeployIntent
	notify   chan struct{}
}

func NewStore() *Store {
	return &Store{
		inFlight: make(map[string]*DeployIntent),
		notify:   make(chan struct{}, 1),
	}
}

// Enqueue appends an intent to the queue and wakes any blocked TakeNext caller.
func (s *Store) Enqueue(i *DeployIntent) {
	s.mu.Lock()
	s.queue = append(s.queue, i)
	s.mu.Unlock()

	// Non-blocking notify; the channel is buffered to depth 1 because TakeNext
	// re-checks the queue under the lock after waking, so coalescing is safe.
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// TakeNext blocks until an intent is available or ctx is cancelled. On success
// the intent is moved from the queue to the in-flight map and its status is
// flipped to StatusDispatched.
func (s *Store) TakeNext(ctx context.Context) (*DeployIntent, error) {
	for {
		s.mu.Lock()
		if len(s.queue) > 0 {
			i := s.queue[0]
			s.queue = s.queue[1:]
			i.Status = StatusDispatched
			s.inFlight[i.ID] = i
			s.mu.Unlock()
			return i, nil
		}
		s.mu.Unlock()

		select {
		case <-s.notify:
			// loop and re-check
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Complete removes an intent from the in-flight map and returns it together
// with a found flag. The caller is responsible for any GitHub side effects.
func (s *Store) Complete(id string, success bool) (*DeployIntent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i, ok := s.inFlight[id]
	if !ok {
		return nil, false
	}
	delete(s.inFlight, id)
	if success {
		i.Status = StatusSucceeded
	} else {
		i.Status = StatusFailed
	}
	return i, true
}

// List returns a shallow copy of the queued intents (not in-flight).
func (s *Store) List() []*DeployIntent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*DeployIntent, len(s.queue))
	copy(out, s.queue)
	return out
}

// InFlight returns a shallow copy of the intents currently with an agent.
func (s *Store) InFlight() []*DeployIntent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*DeployIntent, 0, len(s.inFlight))
	for _, i := range s.inFlight {
		out = append(out, i)
	}
	return out
}

func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue)
}
