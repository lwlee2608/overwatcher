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
	s.Enqueue(&DeployIntent{ID: "a", Stack: "s1"})
	s.Enqueue(&DeployIntent{ID: "b", Stack: "s2"})
	s.Enqueue(&DeployIntent{ID: "c", Stack: "s3"})

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

	s.Enqueue(&DeployIntent{ID: "late", Stack: "s1"})

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
	s.Enqueue(&DeployIntent{ID: "a", Stack: "s1"})

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
		s.Enqueue(&DeployIntent{ID: "b", Stack: "s2"})
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

func TestStore_TakeNext_SetsAttemptsAndDispatchedAt(t *testing.T) {
	s := NewStore()
	s.Enqueue(&DeployIntent{ID: "a", Stack: "s1"})

	i, err := s.TakeNext(context.Background())
	if err != nil {
		t.Fatalf("TakeNext: %v", err)
	}
	if i.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", i.Attempts)
	}
	if i.DispatchedAt.IsZero() {
		t.Error("DispatchedAt is zero")
	}
}

func TestStore_TakeNext_ConcurrencyGuard_SameStack(t *testing.T) {
	s := NewStore()
	s.Enqueue(&DeployIntent{ID: "a", Stack: "s1"})
	s.Enqueue(&DeployIntent{ID: "b", Stack: "s1"})

	// First take succeeds.
	first, err := s.TakeNext(context.Background())
	if err != nil {
		t.Fatalf("TakeNext: %v", err)
	}
	if first.ID != "a" {
		t.Fatalf("got %q, want a", first.ID)
	}

	// Second take should block because s1 is in flight. Use a short timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = s.TakeNext(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}

	// Complete the first — second should now be available.
	s.Complete(first.ID, true)
	second, err := s.TakeNext(context.Background())
	if err != nil {
		t.Fatalf("TakeNext after Complete: %v", err)
	}
	if second.ID != "b" {
		t.Errorf("got %q, want b", second.ID)
	}
}

func TestStore_TakeNext_ConcurrencyGuard_DifferentStacks(t *testing.T) {
	s := NewStore()
	s.Enqueue(&DeployIntent{ID: "a", Stack: "s1"})
	s.Enqueue(&DeployIntent{ID: "b", Stack: "s2"})

	first, err := s.TakeNext(context.Background())
	if err != nil {
		t.Fatalf("TakeNext 1: %v", err)
	}
	second, err := s.TakeNext(context.Background())
	if err != nil {
		t.Fatalf("TakeNext 2: %v", err)
	}
	if first.ID != "a" || second.ID != "b" {
		t.Errorf("got (%q, %q), want (a, b)", first.ID, second.ID)
	}
}

func TestStore_Complete_WakesTakeNext(t *testing.T) {
	s := NewStore()
	s.Enqueue(&DeployIntent{ID: "a", Stack: "s1"})
	s.Enqueue(&DeployIntent{ID: "b", Stack: "s1"})

	first, _ := s.TakeNext(context.Background())

	done := make(chan *DeployIntent, 1)
	go func() {
		i, _ := s.TakeNext(context.Background())
		done <- i
	}()

	// Give goroutine time to park.
	time.Sleep(20 * time.Millisecond)
	s.Complete(first.ID, true)

	select {
	case i := <-done:
		if i.ID != "b" {
			t.Errorf("got %q, want b", i.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("TakeNext did not wake after Complete")
	}
}

func TestStore_Requeue(t *testing.T) {
	s := NewStore()
	s.Enqueue(&DeployIntent{ID: "a", Stack: "s1"})
	s.Enqueue(&DeployIntent{ID: "b", Stack: "s2"})

	taken, _ := s.TakeNext(context.Background())
	if taken.ID != "a" {
		t.Fatalf("got %q, want a", taken.ID)
	}

	i, ok := s.Requeue(taken.ID)
	if !ok {
		t.Fatal("Requeue returned not found")
	}
	if i.Status != StatusCreated {
		t.Errorf("status = %q, want StatusCreated", i.Status)
	}
	if len(s.InFlight()) != 0 {
		t.Errorf("InFlight = %d, want 0", len(s.InFlight()))
	}

	// Requeued intent should be at the front.
	next, _ := s.TakeNext(context.Background())
	if next.ID != "a" {
		t.Errorf("got %q, want a (front of queue)", next.ID)
	}
}

func TestStore_SweepTimedOut_Requeue(t *testing.T) {
	s := NewStore()
	s.Enqueue(&DeployIntent{ID: "a", Stack: "s1"})
	i, _ := s.TakeNext(context.Background())

	// Backdate DispatchedAt.
	s.TestSetInFlight(i.ID, func(di *DeployIntent) {
		di.DispatchedAt = time.Now().Add(-15 * time.Minute)
	})

	requeued, failed := s.SweepTimedOut(10*time.Minute, 3)
	if len(requeued) != 1 {
		t.Fatalf("requeued = %d, want 1", len(requeued))
	}
	if len(failed) != 0 {
		t.Fatalf("failed = %d, want 0", len(failed))
	}
	if requeued[0].Status != StatusCreated {
		t.Errorf("status = %q, want StatusCreated", requeued[0].Status)
	}
	if s.Len() != 1 {
		t.Errorf("queue Len = %d, want 1", s.Len())
	}
}

func TestStore_SweepTimedOut_PermanentFailure(t *testing.T) {
	s := NewStore()
	s.Enqueue(&DeployIntent{ID: "a", Stack: "s1"})
	i, _ := s.TakeNext(context.Background())

	// Simulate prior attempts and backdate.
	s.TestSetInFlight(i.ID, func(di *DeployIntent) {
		di.Attempts = 3
		di.DispatchedAt = time.Now().Add(-15 * time.Minute)
	})

	requeued, failed := s.SweepTimedOut(10*time.Minute, 3)
	if len(requeued) != 0 {
		t.Fatalf("requeued = %d, want 0", len(requeued))
	}
	if len(failed) != 1 {
		t.Fatalf("failed = %d, want 1", len(failed))
	}
	if failed[0].Status != StatusPermanentlyFailed {
		t.Errorf("status = %q, want StatusPermanentlyFailed", failed[0].Status)
	}
}

func TestStore_SweepTimedOut_NotYetTimedOut(t *testing.T) {
	s := NewStore()
	s.Enqueue(&DeployIntent{ID: "a", Stack: "s1"})
	s.TakeNext(context.Background())

	requeued, failed := s.SweepTimedOut(10*time.Minute, 3)
	if len(requeued) != 0 || len(failed) != 0 {
		t.Errorf("expected no sweep results, got requeued=%d failed=%d", len(requeued), len(failed))
	}
	if len(s.InFlight()) != 1 {
		t.Errorf("InFlight = %d, want 1", len(s.InFlight()))
	}
}

func TestStore_ConcurrentTakeAndComplete(t *testing.T) {
	s := NewStore()
	const total = 200

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Producer — each intent gets a unique stack so the concurrency guard
	// doesn't serialize them.
	go func() {
		for i := 0; i < total; i++ {
			s.Enqueue(&DeployIntent{ID: fmt.Sprintf("intent-%d", i), Stack: fmt.Sprintf("stack-%d", i)})
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
