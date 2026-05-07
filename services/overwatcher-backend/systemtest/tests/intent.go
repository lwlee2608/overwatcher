package tests

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lwlee2608/overwatcher/internal/service/intent"
)

func TestIntent(t *testing.T, pool *pgxpool.Pool) {
	t.Run("EnqueueAndLen", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		if s.Len() != 0 {
			t.Fatalf("new store Len = %d, want 0", s.Len())
		}
		s.Enqueue(newIntent("d1", "s1"))
		s.Enqueue(newIntent("d2", "s2"))
		if s.Len() != 2 {
			t.Errorf("Len = %d, want 2", s.Len())
		}
	})

	t.Run("ConcurrentEnqueue", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		const workers = 10
		const perWorker = 5

		var wg sync.WaitGroup
		wg.Add(workers)
		for w := 0; w < workers; w++ {
			go func(w int) {
				defer wg.Done()
				for i := 0; i < perWorker; i++ {
					s.Enqueue(newIntent(
						fmt.Sprintf("d-%d-%d", w, i),
						fmt.Sprintf("s-%d-%d", w, i),
					))
				}
			}(w)
		}
		wg.Wait()

		if got := s.Len(); got != workers*perWorker {
			t.Errorf("Len = %d, want %d", got, workers*perWorker)
		}
	})

	t.Run("TakeNext_FIFO", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		s.Enqueue(newIntent("d1", "s1"))
		s.Enqueue(newIntent("d2", "s2"))
		s.Enqueue(newIntent("d3", "s3"))

		ctx := context.Background()
		for _, want := range []string{"s1", "s2", "s3"} {
			got, err := s.TakeNext(ctx)
			if err != nil {
				t.Fatalf("TakeNext: %v", err)
			}
			if got.Stack != want {
				t.Errorf("got stack %q, want %q", got.Stack, want)
			}
			if got.Status != intent.StatusDispatched {
				t.Errorf("status = %q, want StatusDispatched", got.Status)
			}
		}
		if s.Len() != 0 {
			t.Errorf("queue Len = %d, want 0", s.Len())
		}
		if got := len(s.InFlight()); got != 3 {
			t.Errorf("InFlight = %d, want 3", got)
		}
	})

	t.Run("TakeNext_BlocksUntilEnqueue", func(t *testing.T) {
		s := freshIntentStore(t, pool)

		type result struct {
			intent *intent.DeployIntent
			err    error
		}
		done := make(chan result, 1)
		go func() {
			i, err := s.TakeNext(context.Background())
			done <- result{i, err}
		}()

		select {
		case <-done:
			t.Fatal("TakeNext returned before any enqueue")
		case <-time.After(50 * time.Millisecond):
		}

		s.Enqueue(newIntent("d-late", "s1"))

		select {
		case r := <-done:
			if r.err != nil {
				t.Fatalf("TakeNext: %v", r.err)
			}
			if r.intent.Stack != "s1" {
				t.Errorf("got stack %q, want s1", r.intent.Stack)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("TakeNext did not wake after Enqueue")
		}
	})

	t.Run("TakeNext_CtxCancellation", func(t *testing.T) {
		s := freshIntentStore(t, pool)
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
	})

	t.Run("Complete", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		s.Enqueue(newIntent("d1", "s1"))
		taken, err := s.TakeNext(context.Background())
		if err != nil {
			t.Fatalf("TakeNext: %v", err)
		}

		t.Run("success transitions to StatusSucceeded", func(t *testing.T) {
			i, ok := s.Complete(taken.ID, true)
			if !ok {
				t.Fatal("Complete returned not found")
			}
			if i.Status != intent.StatusSucceeded {
				t.Errorf("status = %q, want StatusSucceeded", i.Status)
			}
			if got := len(s.InFlight()); got != 0 {
				t.Errorf("InFlight = %d, want 0", got)
			}
		})

		t.Run("unknown id returns false", func(t *testing.T) {
			if _, ok := s.Complete("00000000-0000-0000-0000-000000000000", true); ok {
				t.Error("Complete returned true for unknown id")
			}
		})

		t.Run("failure transitions to StatusFailed", func(t *testing.T) {
			s.Enqueue(newIntent("d2", "s2"))
			b, err := s.TakeNext(context.Background())
			if err != nil {
				t.Fatalf("TakeNext: %v", err)
			}
			i, ok := s.Complete(b.ID, false)
			if !ok {
				t.Fatal("Complete returned not found")
			}
			if i.Status != intent.StatusFailed {
				t.Errorf("status = %q, want StatusFailed", i.Status)
			}
		})
	})

	t.Run("TakeNext_SetsAttemptsAndDispatchedAt", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		s.Enqueue(newIntent("d1", "s1"))
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
	})

	t.Run("ConcurrencyGuard_SameStack", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		s.Enqueue(newIntent("d1", "s1"))
		s.Enqueue(newIntent("d2", "s1"))

		first, err := s.TakeNext(context.Background())
		if err != nil {
			t.Fatalf("TakeNext: %v", err)
		}
		if first.DeliveryID != "d1" {
			t.Fatalf("got %q, want d1", first.DeliveryID)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		if _, err := s.TakeNext(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected DeadlineExceeded, got %v", err)
		}

		s.Complete(first.ID, true)
		second, err := s.TakeNext(context.Background())
		if err != nil {
			t.Fatalf("TakeNext after Complete: %v", err)
		}
		if second.DeliveryID != "d2" {
			t.Errorf("got %q, want d2", second.DeliveryID)
		}
	})

	t.Run("ConcurrencyGuard_DifferentStacks", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		s.Enqueue(newIntent("d1", "s1"))
		s.Enqueue(newIntent("d2", "s2"))

		first, err := s.TakeNext(context.Background())
		if err != nil {
			t.Fatalf("TakeNext 1: %v", err)
		}
		second, err := s.TakeNext(context.Background())
		if err != nil {
			t.Fatalf("TakeNext 2: %v", err)
		}
		if first.DeliveryID != "d1" || second.DeliveryID != "d2" {
			t.Errorf("got (%q, %q), want (d1, d2)", first.DeliveryID, second.DeliveryID)
		}
	})

	t.Run("Complete_WakesTakeNext", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		s.Enqueue(newIntent("d1", "s1"))
		s.Enqueue(newIntent("d2", "s1"))

		first, _ := s.TakeNext(context.Background())

		done := make(chan *intent.DeployIntent, 1)
		go func() {
			i, _ := s.TakeNext(context.Background())
			done <- i
		}()

		time.Sleep(50 * time.Millisecond)
		s.Complete(first.ID, true)

		select {
		case i := <-done:
			if i.DeliveryID != "d2" {
				t.Errorf("got %q, want d2", i.DeliveryID)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("TakeNext did not wake after Complete")
		}
	})

	t.Run("Requeue", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		s.Enqueue(newIntent("d1", "s1"))
		s.Enqueue(newIntent("d2", "s2"))

		taken, _ := s.TakeNext(context.Background())
		if taken.DeliveryID != "d1" {
			t.Fatalf("got %q, want d1", taken.DeliveryID)
		}
		i, ok := s.Requeue(taken.ID)
		if !ok {
			t.Fatal("Requeue returned not found")
		}
		if i.Status != intent.StatusCreated {
			t.Errorf("status = %q, want StatusCreated", i.Status)
		}
		if len(s.InFlight()) != 0 {
			t.Errorf("InFlight = %d, want 0", len(s.InFlight()))
		}
		next, _ := s.TakeNext(context.Background())
		if next.DeliveryID != "d1" {
			t.Errorf("got %q, want d1", next.DeliveryID)
		}
	})

	t.Run("SweepTimedOut_Requeue", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		s.Enqueue(newIntent("d1", "s1"))
		i, _ := s.TakeNext(context.Background())

		backdateInFlight(t, pool, i.ID, time.Now().Add(-15*time.Minute), 1)

		requeued, failed := s.SweepTimedOut(10*time.Minute, 3)
		if len(requeued) != 1 {
			t.Fatalf("requeued = %d, want 1", len(requeued))
		}
		if len(failed) != 0 {
			t.Fatalf("failed = %d, want 0", len(failed))
		}
		if requeued[0].Status != intent.StatusCreated {
			t.Errorf("status = %q, want StatusCreated", requeued[0].Status)
		}
		if s.Len() != 1 {
			t.Errorf("Len = %d, want 1", s.Len())
		}
	})

	t.Run("SweepTimedOut_PermanentFailure", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		s.Enqueue(newIntent("d1", "s1"))
		i, _ := s.TakeNext(context.Background())

		backdateInFlight(t, pool, i.ID, time.Now().Add(-15*time.Minute), 3)

		requeued, failed := s.SweepTimedOut(10*time.Minute, 3)
		if len(requeued) != 0 {
			t.Fatalf("requeued = %d, want 0", len(requeued))
		}
		if len(failed) != 1 {
			t.Fatalf("failed = %d, want 1", len(failed))
		}
		if failed[0].Status != intent.StatusPermanentlyFailed {
			t.Errorf("status = %q, want StatusPermanentlyFailed", failed[0].Status)
		}
	})

	t.Run("SweepTimedOut_NotYetTimedOut", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		s.Enqueue(newIntent("d1", "s1"))
		if _, err := s.TakeNext(context.Background()); err != nil {
			t.Fatalf("TakeNext: %v", err)
		}
		requeued, failed := s.SweepTimedOut(10*time.Minute, 3)
		if len(requeued) != 0 || len(failed) != 0 {
			t.Errorf("expected no sweep results, got requeued=%d failed=%d", len(requeued), len(failed))
		}
		if len(s.InFlight()) != 1 {
			t.Errorf("InFlight = %d, want 1", len(s.InFlight()))
		}
	})

	t.Run("ConcurrentTakeAndComplete", func(t *testing.T) {
		s := freshIntentStore(t, pool)
		const total = 50

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		go func() {
			for i := 0; i < total; i++ {
				s.Enqueue(newIntent(fmt.Sprintf("d-%d", i), fmt.Sprintf("stack-%d", i)))
			}
		}()

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
			t.Errorf("InFlight = %d, want 0", got)
		}
	})
}

func freshIntentStore(t *testing.T, pool *pgxpool.Pool) *intent.DBStore {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, "TRUNCATE deploy_intents"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return intent.NewDBStore(pool)
}

func newIntent(deliveryID, stack string) *intent.DeployIntent {
	return &intent.DeployIntent{
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

// .UTC() is load-bearing: pgtype.Timestamp's discardTimeZone preserves
// wall-clock digits and relabels them as UTC, which shifts the value by
// the local offset on non-UTC hosts.
func backdateInFlight(t *testing.T, pool *pgxpool.Pool, id string, dispatchedAt time.Time, attempts int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := pool.Exec(ctx,
		`UPDATE deploy_intents SET dispatched_at = $1, attempts = $2 WHERE id::text = $3`,
		dispatchedAt.UTC(), attempts, id)
	if err != nil {
		t.Fatalf("backdate: %v", err)
	}
}
