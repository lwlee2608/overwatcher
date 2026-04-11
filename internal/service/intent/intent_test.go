package intent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestStore_EnqueueAndLen(t *testing.T) {
	s := NewStore()
	if s.Len() != 0 {
		t.Fatalf("new store Len = %d, want 0", s.Len())
	}

	s.Enqueue(&DeployIntent{ID: "a"})
	s.Enqueue(&DeployIntent{ID: "b"})

	if s.Len() != 2 {
		t.Errorf("Len = %d, want 2", s.Len())
	}
}

func TestStore_ListReturnsCopy(t *testing.T) {
	s := NewStore()
	s.Enqueue(&DeployIntent{ID: "a"})

	list := s.List()
	list = append(list, &DeployIntent{ID: "rogue"})

	if s.Len() != 1 {
		t.Errorf("external append leaked into store: Len = %d, want 1", s.Len())
	}
}

func TestStore_ConcurrentEnqueue(t *testing.T) {
	s := NewStore()
	const workers = 50
	const perWorker = 20

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				s.Enqueue(&DeployIntent{ID: fmt.Sprintf("%d-%d", w, i)})
			}
		}(w)
	}
	wg.Wait()

	if got := s.Len(); got != workers*perWorker {
		t.Errorf("Len = %d, want %d", got, workers*perWorker)
	}
}

func TestStore_TakeNext_FIFO(t *testing.T) {
	s := NewStore()
	s.Enqueue(&DeployIntent{ID: "a"})
	s.Enqueue(&DeployIntent{ID: "b"})
	s.Enqueue(&DeployIntent{ID: "c"})

	ctx := context.Background()
	for _, want := range []string{"a", "b", "c"} {
		got, err := s.TakeNext(ctx)
		if err != nil {
			t.Fatalf("TakeNext error: %v", err)
		}
		if got.ID != want {
			t.Errorf("got %q, want %q", got.ID, want)
		}
		if got.Status != StatusDispatched {
			t.Errorf("status = %q, want StatusDispatched", got.Status)
		}
	}

	if s.Len() != 0 {
		t.Errorf("queue Len = %d, want 0", s.Len())
	}
	if got := len(s.InFlight()); got != 3 {
		t.Errorf("InFlight len = %d, want 3", got)
	}
}

func TestStore_TakeNext_BlocksUntilEnqueue(t *testing.T) {
	s := NewStore()

	type result struct {
		intent *DeployIntent
		err    error
	}
	done := make(chan result, 1)
	go func() {
		i, err := s.TakeNext(context.Background())
		done <- result{i, err}
	}()

	// Give the goroutine time to park on the notify channel.
	select {
	case <-done:
		t.Fatal("TakeNext returned before any enqueue")
	case <-time.After(20 * time.Millisecond):
	}

	s.Enqueue(&DeployIntent{ID: "late"})

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("TakeNext error: %v", r.err)
		}
		if r.intent.ID != "late" {
			t.Errorf("got %q, want late", r.intent.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("TakeNext did not wake after Enqueue")
	}
}

func TestStore_TakeNext_CtxCancellation(t *testing.T) {
	s := NewStore()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := s.TakeNext(ctx)
		done <- err
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("TakeNext did not return after ctx cancel")
	}
}

func TestStore_Complete(t *testing.T) {
	s := NewStore()
	s.Enqueue(&DeployIntent{ID: "a"})

	taken, err := s.TakeNext(context.Background())
	if err != nil {
		t.Fatalf("TakeNext: %v", err)
	}

	t.Run("success transitions to StatusSucceeded", func(t *testing.T) {
		i, ok := s.Complete(taken.ID, true)
		if !ok {
			t.Fatal("Complete returned not found")
		}
		if i.Status != StatusSucceeded {
			t.Errorf("status = %q, want StatusSucceeded", i.Status)
		}
		if got := len(s.InFlight()); got != 0 {
			t.Errorf("InFlight len = %d, want 0", got)
		}
	})

	t.Run("unknown id returns false", func(t *testing.T) {
		if _, ok := s.Complete("does-not-exist", true); ok {
			t.Error("Complete returned true for unknown id")
		}
	})

	t.Run("failure transitions to StatusFailed", func(t *testing.T) {
		s.Enqueue(&DeployIntent{ID: "b"})
		_, _ = s.TakeNext(context.Background())
		i, ok := s.Complete("b", false)
		if !ok {
			t.Fatal("Complete returned not found")
		}
		if i.Status != StatusFailed {
			t.Errorf("status = %q, want StatusFailed", i.Status)
		}
	})
}

func TestStore_ConcurrentTakeAndComplete(t *testing.T) {
	s := NewStore()
	const total = 200

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Producer.
	go func() {
		for i := 0; i < total; i++ {
			s.Enqueue(&DeployIntent{ID: fmt.Sprintf("intent-%d", i)})
		}
	}()

	// Consumers. Each worker blocks in TakeNext until either an intent is
	// available or ctx is cancelled. The first worker to push the counter to
	// total cancels ctx so the other workers wake immediately instead of
	// hanging until the timeout.
	const workers = 8
	var taken int64
	var wg sync.WaitGroup
	wg.Add(workers)

	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for {
				i, err := s.TakeNext(ctx)
				if err != nil {
					return
				}
				if _, ok := s.Complete(i.ID, true); !ok {
					t.Errorf("Complete returned not found for %s", i.ID)
					return
				}
				if atomic.AddInt64(&taken, 1) >= total {
					cancel()
				}
			}
		}()
	}

	wg.Wait()

	if got := atomic.LoadInt64(&taken); got != total {
		t.Errorf("taken = %d, want %d", got, total)
	}
	if got := s.Len(); got != 0 {
		t.Errorf("queue Len = %d, want 0", got)
	}
	if got := len(s.InFlight()); got != 0 {
		t.Errorf("InFlight len = %d, want 0", got)
	}
}
