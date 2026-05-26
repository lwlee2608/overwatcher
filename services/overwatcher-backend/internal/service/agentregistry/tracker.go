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

var (
	ErrNotFound = errors.New("agent not found")
	ErrBound    = errors.New("agent is bound to a project")
)

// ConnectionStatus describes how recently an agent has heartbeated.
//
//	connected:    age < staleAfter                          — healthy
//	stale:        staleAfter ≤ age < disconnectedAfter      — missed a few polls
//	disconnected: disconnectedAfter ≤ age < lostAfter       — assume gone
//	lost:         age ≥ lostAfter                           — long gone, likely abandoned
type ConnectionStatus string

const (
	StatusConnected    ConnectionStatus = "connected"
	StatusStale        ConnectionStatus = "stale"
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusLost         ConnectionStatus = "lost"
)

// AgentStatus is a point-in-time snapshot of a registered agent.
type AgentStatus struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	LastSeen  time.Time        `json:"last_seen"`
	RemoteIP  string           `json:"remote_ip"`
	Status    ConnectionStatus `json:"status"`
	ProjectID string           `json:"project_id,omitempty"`
	Type      string           `json:"type,omitempty"`
	Version   string           `json:"version,omitempty"`
}

// Thresholds defines the age cutoffs (now − last_seen) used to derive
// ConnectionStatus. Must satisfy StaleAfter ≤ DisconnectedAfter ≤ LostAfter.
type Thresholds struct {
	StaleAfter        time.Duration
	DisconnectedAfter time.Duration
	LostAfter         time.Duration
}

// Service manages agent registration and heartbeats backed by PostgreSQL.
type Service struct {
	pool       *pgxpool.Pool
	q          *sqlc.Queries
	thresholds Thresholds
}

// NewService constructs the registry.
func NewService(pool *pgxpool.Pool, q *sqlc.Queries, thresholds Thresholds) *Service {
	return &Service{
		pool:       pool,
		q:          q,
		thresholds: thresholds,
	}
}

// Record upserts an agent entry with the current time. Empty agentType or
// version leaves any existing stored value intact (see UpsertAgent SQL).
func (s *Service) Record(ctx context.Context, name string, remoteIP string, agentType string, version string) error {
	_, err := s.q.UpsertAgent(ctx, sqlc.UpsertAgentParams{
		Name:      name,
		RemoteIp:  remoteIP,
		AgentType: agentType,
		Version:   version,
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

// Delete removes an agent record. Returns ErrBound if the agent is currently
// bound to a project — caller must unbind first. A live agent will simply
// re-register on its next heartbeat (new ID, no project binding).
func (s *Service) Delete(ctx context.Context, id string) error {
	aid := pgtype.UUID{}
	if err := aid.Scan(id); err != nil {
		return err
	}
	a, err := s.q.GetAgent(ctx, aid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if a.ProjectID.Valid {
		return ErrBound
	}
	n, err := s.q.DeleteAgent(ctx, aid)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) toStatus(a sqlc.Agent, now time.Time) AgentStatus {
	lastSeen := a.LastSeenAt.Time
	age := now.Sub(lastSeen)
	var status ConnectionStatus
	switch {
	case age < s.thresholds.StaleAfter:
		status = StatusConnected
	case age < s.thresholds.DisconnectedAfter:
		status = StatusStale
	case age < s.thresholds.LostAfter:
		status = StatusDisconnected
	default:
		status = StatusLost
	}
	return AgentStatus{
		ID:        util.UUIDToString(a.ID),
		Name:      a.Name,
		LastSeen:  lastSeen,
		RemoteIP:  a.RemoteIp,
		Status:    status,
		ProjectID: util.UUIDToString(a.ProjectID),
		Type:      a.AgentType.String,
		Version:   a.Version.String,
	}
}
