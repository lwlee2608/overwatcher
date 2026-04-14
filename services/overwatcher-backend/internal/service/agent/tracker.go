package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
)

// AgentStatus is a point-in-time snapshot of a registered agent.
type AgentStatus struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ComposeFile string    `json:"compose_file"`
	LastSeen    time.Time `json:"last_seen"`
	RemoteIP    string    `json:"remote_ip"`
	Connected   bool      `json:"connected"`
}

// Service manages agent registration and heartbeats backed by PostgreSQL.
type Service struct {
	q   *sqlc.Queries
	ttl time.Duration
}

func NewService(q *sqlc.Queries, ttl time.Duration) *Service {
	return &Service{q: q, ttl: ttl}
}

// Record upserts an agent entry with the current time.
func (s *Service) Record(ctx context.Context, name string, composeFile string, remoteIP string) error {
	_, err := s.q.UpsertAgent(ctx, sqlc.UpsertAgentParams{
		Name:        name,
		ComposeFile: composeFile,
		RemoteIp:    remoteIP,
	})
	return err
}

// List returns a snapshot of all known agents, sorted by name.
func (s *Service) List(ctx context.Context) ([]AgentStatus, error) {
	agents, err := s.q.ListAgents(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	out := make([]AgentStatus, len(agents))
	for i, a := range agents {
		out[i] = s.toStatus(a, now)
	}
	return out, nil
}

// GetByID returns a single agent by its UUID.
func (s *Service) GetByID(ctx context.Context, id string) (*AgentStatus, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return nil, err
	}
	a, err := s.q.GetAgent(ctx, uid)
	if err != nil {
		return nil, err
	}
	status := s.toStatus(a, time.Now())
	return &status, nil
}

func (s *Service) toStatus(a sqlc.Agent, now time.Time) AgentStatus {
	lastSeen := a.LastSeenAt.Time
	return AgentStatus{
		ID:          uuidToString(a.ID),
		Name:        a.Name,
		ComposeFile: a.ComposeFile,
		LastSeen:    lastSeen,
		RemoteIP:    a.RemoteIp,
		Connected:   now.Sub(lastSeen) < s.ttl,
	}
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
