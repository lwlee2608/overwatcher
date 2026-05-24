package agentregistry

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
	"github.com/lwlee2608/overwatcher/internal/util"
)

var ErrNotFound = errors.New("agent not found")

// AgentStatus is a point-in-time snapshot of a registered agent.
type AgentStatus struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	LastSeen  time.Time `json:"last_seen"`
	RemoteIP  string    `json:"remote_ip"`
	Connected bool      `json:"connected"`
	ProjectID string    `json:"project_id,omitempty"`
	Type      string    `json:"type,omitempty"`
}

// Service manages agent registration and heartbeats backed by PostgreSQL.
type Service struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
	ttl  time.Duration
}

func NewService(pool *pgxpool.Pool, q *sqlc.Queries, ttl time.Duration) *Service {
	return &Service{pool: pool, q: q, ttl: ttl}
}

// Record upserts an agent entry with the current time. An empty agentType
// leaves any existing stored type intact (see UpsertAgent SQL).
func (s *Service) Record(ctx context.Context, name string, remoteIP string, agentType string) error {
	_, err := s.q.UpsertAgent(ctx, sqlc.UpsertAgentParams{
		Name:      name,
		RemoteIp:  remoteIP,
		AgentType: agentType,
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

// GetByName returns a single agent by its name. Returns ErrNotFound when no
// agent with that name exists.
func (s *Service) GetByName(ctx context.Context, name string) (*AgentStatus, error) {
	a, err := s.q.GetAgentByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	status := s.toStatus(a, time.Now())
	return &status, nil
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

// BindProject sets (or clears, when projectID is empty) the project binding on an agent.
// If projectID is non-empty, any other agent currently bound to that project is
// cleared first so the 1:1 constraint is preserved.
func (s *Service) BindProject(ctx context.Context, agentID string, projectID string) (*AgentStatus, error) {
	aid := pgtype.UUID{}
	if err := aid.Scan(agentID); err != nil {
		return nil, err
	}
	pid := pgtype.UUID{}
	if projectID != "" {
		if err := pid.Scan(projectID); err != nil {
			return nil, err
		}
	}

	// Unbinding does not need a transaction.
	if projectID == "" {
		a, err := s.q.BindAgentToProject(ctx, sqlc.BindAgentToProjectParams{
			ID:        aid,
			ProjectID: pid,
		})
		if err != nil {
			return nil, err
		}
		status := s.toStatus(a, time.Now())
		return &status, nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	q := s.q.WithTx(tx)

	if err = q.ClearAgentProjectBinding(ctx, pid); err != nil {
		return nil, err
	}

	a, err := q.BindAgentToProject(ctx, sqlc.BindAgentToProjectParams{
		ID:        aid,
		ProjectID: pid,
	})
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	status := s.toStatus(a, time.Now())
	return &status, nil
}

func (s *Service) toStatus(a sqlc.Agent, now time.Time) AgentStatus {
	lastSeen := a.LastSeenAt.Time
	return AgentStatus{
		ID:        util.UUIDToString(a.ID),
		Name:      a.Name,
		LastSeen:  lastSeen,
		RemoteIP:  a.RemoteIp,
		Connected: now.Sub(lastSeen) < s.ttl,
		ProjectID: util.UUIDToString(a.ProjectID),
		Type:      a.AgentType.String,
	}
}

