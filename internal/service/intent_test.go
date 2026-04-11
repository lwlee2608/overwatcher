package service

import (
	"fmt"
	"sync"
	"testing"
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
