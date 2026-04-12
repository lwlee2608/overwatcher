package agent

import (
	"sort"
	"sync"
	"time"
)

// AgentStatus is a point-in-time snapshot of a connected agent.
type AgentStatus struct {
	Name      string   `json:"name"`
	Stacks    []string `json:"stacks"`
	LastSeen  time.Time `json:"last_seen"`
	RemoteIP  string   `json:"remote_ip"`
	Connected bool     `json:"connected"`
}

type agentInfo struct {
	stacks   []string
	lastSeen time.Time
	remoteIP string
}

// Tracker keeps an in-memory record of agents that have recently polled.
// Each poll is treated as an implicit heartbeat.
type Tracker struct {
	mu     sync.RWMutex
	agents map[string]*agentInfo
	ttl    time.Duration
}

func NewTracker(ttl time.Duration) *Tracker {
	return &Tracker{
		agents: make(map[string]*agentInfo),
		ttl:    ttl,
	}
}

// Record upserts an agent entry with the current time.
func (t *Tracker) Record(name string, stacks []string, remoteIP string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.agents[name] = &agentInfo{
		stacks:   stacks,
		lastSeen: time.Now(),
		remoteIP: remoteIP,
	}
}

// List returns a snapshot of all known agents, sorted by name.
func (t *Tracker) List() []AgentStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now()
	out := make([]AgentStatus, 0, len(t.agents))
	for name, info := range t.agents {
		out = append(out, AgentStatus{
			Name:      name,
			Stacks:    append([]string(nil), info.stacks...),
			LastSeen:  info.lastSeen,
			RemoteIP:  info.remoteIP,
			Connected: now.Sub(info.lastSeen) < t.ttl,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
