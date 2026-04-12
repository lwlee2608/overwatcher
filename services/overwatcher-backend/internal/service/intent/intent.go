package intent

import (
	"context"
	"sync"
	"time"
)

// Status tracks a DeployIntent through its lifecycle.
type Status string

const (
	StatusCreated           Status = "created"
	StatusDispatched        Status = "dispatched"
	StatusSucceeded         Status = "succeeded"
	StatusFailed            Status = "failed"
	StatusPermanentlyFailed Status = "permanently_failed"
)

// DeployIntent is everything an agent needs to perform a deploy. It is produced
// on a push webhook when the repo matches a mapping.Entry.
type DeployIntent struct {
	ID             string
	CreatedAt      time.Time
	DeliveryID     string
	StackIndex     int
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
	Attempts       int
	DispatchedAt   time.Time
}

var _ Store = (*MemoryStore)(nil)

// MemoryStore is an in-memory, thread-safe queue of DeployIntents plus an
// in-flight map for intents currently being executed by an agent. TakeNext
// blocks until an intent is enqueued or the caller's context is cancelled.
type MemoryStore struct {
	mu       sync.Mutex
	queue    []*DeployIntent
	inFlight map[string]*DeployIntent
	notify   chan struct{}
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		inFlight: make(map[string]*DeployIntent),
		notify:   make(chan struct{}, 1),
	}
}

// Enqueue appends an intent to the queue and wakes any blocked TakeNext caller.
func (s *MemoryStore) Enqueue(i *DeployIntent) {
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

// TakeNext blocks until a dispatchable intent is available or ctx is cancelled.
// An intent is dispatchable when no other intent for the same stack is already
// in flight (concurrency guard). On success the intent is moved from the queue
// to the in-flight map, its status is flipped to StatusDispatched, Attempts is
// incremented, and DispatchedAt is set.
func (s *MemoryStore) TakeNext(ctx context.Context) (*DeployIntent, error) {
	for {
		s.mu.Lock()
		for idx, i := range s.queue {
			if !s.stackInFlight(i.Stack) {
				s.queue = append(s.queue[:idx], s.queue[idx+1:]...)
				i.Status = StatusDispatched
				i.Attempts++
				i.DispatchedAt = time.Now()
				s.inFlight[i.ID] = i
				s.mu.Unlock()
				return i, nil
			}
		}
		s.mu.Unlock()

		select {
		case <-s.notify:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// stackInFlight reports whether any in-flight intent targets the given stack.
// Must be called with s.mu held.
func (s *MemoryStore) stackInFlight(stack string) bool {
	for _, i := range s.inFlight {
		if i.Stack == stack {
			return true
		}
	}
	return false
}

// Complete removes an intent from the in-flight map and returns it together
// with a found flag. It also wakes any blocked TakeNext callers so the
// concurrency guard can re-evaluate the queue. The caller is responsible for
// any GitHub side effects.
func (s *MemoryStore) Complete(id string, success bool) (*DeployIntent, bool) {
	s.mu.Lock()
	i, ok := s.inFlight[id]
	if !ok {
		s.mu.Unlock()
		return nil, false
	}
	delete(s.inFlight, id)
	if success {
		i.Status = StatusSucceeded
	} else {
		i.Status = StatusFailed
	}
	s.mu.Unlock()

	// Wake blocked TakeNext callers — a stack slot just freed up.
	select {
	case s.notify <- struct{}{}:
	default:
	}
	return i, true
}

// Requeue moves an intent from in-flight back to the front of the queue with
// StatusCreated. Returns false if the id is not in flight.
func (s *MemoryStore) Requeue(id string) (*DeployIntent, bool) {
	s.mu.Lock()
	i, ok := s.inFlight[id]
	if !ok {
		s.mu.Unlock()
		return nil, false
	}
	delete(s.inFlight, id)
	i.Status = StatusCreated
	s.queue = append([]*DeployIntent{i}, s.queue...)
	s.mu.Unlock()

	select {
	case s.notify <- struct{}{}:
	default:
	}
	return i, true
}

// SweepTimedOut checks all in-flight intents. Those dispatched longer than
// timeout ago are either requeued (if Attempts < maxAttempts) or permanently
// failed. Returns the two sets so the caller can update GitHub accordingly.
func (s *MemoryStore) SweepTimedOut(timeout time.Duration, maxAttempts int) (requeued, failed []*DeployIntent) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, i := range s.inFlight {
		if now.Sub(i.DispatchedAt) <= timeout {
			continue
		}
		delete(s.inFlight, id)
		if i.Attempts >= maxAttempts {
			i.Status = StatusPermanentlyFailed
			failed = append(failed, i)
		} else {
			i.Status = StatusCreated
			s.queue = append([]*DeployIntent{i}, s.queue...)
			requeued = append(requeued, i)
		}
	}

	if len(requeued) > 0 {
		select {
		case s.notify <- struct{}{}:
		default:
		}
	}
	return requeued, failed
}

// List returns a shallow copy of the queued intents (not in-flight).
func (s *MemoryStore) List() []*DeployIntent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*DeployIntent, len(s.queue))
	copy(out, s.queue)
	return out
}

// InFlight returns a shallow copy of the intents currently with an agent.
func (s *MemoryStore) InFlight() []*DeployIntent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*DeployIntent, 0, len(s.inFlight))
	for _, i := range s.inFlight {
		out = append(out, i)
	}
	return out
}

// TestSetInFlight mutates an in-flight intent under the lock. Intended only
// for tests that need to backdate DispatchedAt or adjust Attempts.
func (s *MemoryStore) TestSetInFlight(id string, fn func(*DeployIntent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i, ok := s.inFlight[id]; ok {
		fn(i)
	}
}

func (s *MemoryStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue)
}
