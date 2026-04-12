package agent

import (
	"sync"
	"testing"
	"time"
)

func TestTracker_RecordAndList(t *testing.T) {
	tr := NewTracker(60 * time.Second)
	tr.Record("agent-1", []string{"web", "api"}, "10.0.0.1")

	agents := tr.List()
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	a := agents[0]
	if a.Name != "agent-1" {
		t.Errorf("name = %q, want %q", a.Name, "agent-1")
	}
	if !a.Connected {
		t.Error("expected agent to be connected")
	}
	if a.RemoteIP != "10.0.0.1" {
		t.Errorf("remote_ip = %q, want %q", a.RemoteIP, "10.0.0.1")
	}
	if len(a.Stacks) != 2 || a.Stacks[0] != "web" {
		t.Errorf("stacks = %v, want [web api]", a.Stacks)
	}
}

func TestTracker_Disconnected(t *testing.T) {
	tr := NewTracker(1 * time.Millisecond)
	tr.Record("agent-1", []string{"web"}, "10.0.0.1")

	time.Sleep(5 * time.Millisecond)

	agents := tr.List()
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Connected {
		t.Error("expected agent to be disconnected after TTL")
	}
}

func TestTracker_UpdateExisting(t *testing.T) {
	tr := NewTracker(60 * time.Second)
	tr.Record("agent-1", []string{"web"}, "10.0.0.1")
	tr.Record("agent-1", []string{"web", "worker"}, "10.0.0.2")

	agents := tr.List()
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	a := agents[0]
	if len(a.Stacks) != 2 || a.Stacks[1] != "worker" {
		t.Errorf("stacks = %v, want [web worker]", a.Stacks)
	}
	if a.RemoteIP != "10.0.0.2" {
		t.Errorf("remote_ip = %q, want %q", a.RemoteIP, "10.0.0.2")
	}
}

func TestTracker_ListSortedByName(t *testing.T) {
	tr := NewTracker(60 * time.Second)
	tr.Record("charlie", []string{"c"}, "10.0.0.3")
	tr.Record("alpha", []string{"a"}, "10.0.0.1")
	tr.Record("bravo", []string{"b"}, "10.0.0.2")

	agents := tr.List()
	if len(agents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(agents))
	}
	if agents[0].Name != "alpha" || agents[1].Name != "bravo" || agents[2].Name != "charlie" {
		t.Errorf("agents not sorted: %v, %v, %v", agents[0].Name, agents[1].Name, agents[2].Name)
	}
}

func TestTracker_ConcurrentAccess(t *testing.T) {
	tr := NewTracker(60 * time.Second)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			tr.Record("agent", []string{"web"}, "10.0.0.1")
		}()
		go func() {
			defer wg.Done()
			tr.List()
		}()
	}
	wg.Wait()
}
