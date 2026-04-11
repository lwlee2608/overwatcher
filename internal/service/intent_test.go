package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIntentStore_EnqueueAndLen(t *testing.T) {
	s := NewIntentStore()
	if s.Len() != 0 {
		t.Fatalf("new store Len = %d, want 0", s.Len())
	}

	s.Enqueue(&DeployIntent{ID: "a"})
	s.Enqueue(&DeployIntent{ID: "b"})

	if s.Len() != 2 {
		t.Errorf("Len = %d, want 2", s.Len())
	}
}

func TestIntentStore_ListReturnsCopy(t *testing.T) {
	s := NewIntentStore()
	s.Enqueue(&DeployIntent{ID: "a"})

	list := s.List()
	list = append(list, &DeployIntent{ID: "rogue"})

	if s.Len() != 1 {
		t.Errorf("external append leaked into store: Len = %d, want 1", s.Len())
	}
}

func TestIntentStore_ConcurrentEnqueue(t *testing.T) {
	s := NewIntentStore()
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

func TestIntentStore_TakeNext_FIFO(t *testing.T) {
	s := NewIntentStore()
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
		if got.Status != IntentDispatched {
			t.Errorf("status = %q, want IntentDispatched", got.Status)
		}
	}

	if s.Len() != 0 {
		t.Errorf("queue Len = %d, want 0", s.Len())
	}
	if got := len(s.InFlight()); got != 3 {
		t.Errorf("InFlight len = %d, want 3", got)
	}
}

func TestIntentStore_TakeNext_BlocksUntilEnqueue(t *testing.T) {
	s := NewIntentStore()

	type result struct {
		intent *DeployIntent
		err    error
	}
	done := make(chan result, 1)
	go func() {
		intent, err := s.TakeNext(context.Background())
		done <- result{intent, err}
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

func TestIntentStore_TakeNext_CtxCancellation(t *testing.T) {
	s := NewIntentStore()
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

func TestIntentStore_Complete(t *testing.T) {
	s := NewIntentStore()
	s.Enqueue(&DeployIntent{ID: "a"})

	taken, err := s.TakeNext(context.Background())
	if err != nil {
		t.Fatalf("TakeNext: %v", err)
	}

	t.Run("success transitions to IntentSucceeded", func(t *testing.T) {
		intent, ok := s.Complete(taken.ID, true)
		if !ok {
			t.Fatal("Complete returned not found")
		}
		if intent.Status != IntentSucceeded {
			t.Errorf("status = %q, want IntentSucceeded", intent.Status)
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

	t.Run("failure transitions to IntentFailed", func(t *testing.T) {
		s.Enqueue(&DeployIntent{ID: "b"})
		_, _ = s.TakeNext(context.Background())
		intent, ok := s.Complete("b", false)
		if !ok {
			t.Fatal("Complete returned not found")
		}
		if intent.Status != IntentFailed {
			t.Errorf("status = %q, want IntentFailed", intent.Status)
		}
	})
}

func TestIntentStore_ConcurrentTakeAndComplete(t *testing.T) {
	s := NewIntentStore()
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
				intent, err := s.TakeNext(ctx)
				if err != nil {
					return
				}
				if _, ok := s.Complete(intent.ID, true); !ok {
					t.Errorf("Complete returned not found for %s", intent.ID)
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
