package agentregistry

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
	"github.com/lwlee2608/overwatcher/internal/util"
)

var (
	ErrNotFound  = errors.New("agent not found")
	ErrBound     = errors.New("agent is bound to a project")
	ErrNameTaken = errors.New("agent name already in use")
	ErrBadToken  = errors.New("malformed agent token")
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

// HostMetrics is the last resource snapshot an agent reported with a
// heartbeat. Disk figures are for the agent's root filesystem. SwapTotalBytes
// is 0 when the host has no swap configured.
type HostMetrics struct {
	CPUPercent     float64 `json:"cpu_percent"`
	MemUsedBytes   uint64  `json:"mem_used_bytes"`
	MemTotalBytes  uint64  `json:"mem_total_bytes"`
	SwapUsedBytes  uint64  `json:"swap_used_bytes"`
	SwapTotalBytes uint64  `json:"swap_total_bytes"`
	DiskUsedBytes  uint64  `json:"disk_used_bytes"`
	DiskTotalBytes uint64  `json:"disk_total_bytes"`
}

// AgentStatus is a point-in-time snapshot of a registered agent.
type AgentStatus struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	LastSeen    *time.Time       `json:"last_seen,omitempty"` // LastSeen is nil until the agent's first heartbeat (never connected).
	RemoteIP    string           `json:"remote_ip"`
	Status      ConnectionStatus `json:"status"`
	ProjectID   string           `json:"project_id,omitempty"`
	ProjectName string           `json:"project_name,omitempty"`
	Type        string           `json:"type,omitempty"`
	Version     string           `json:"version,omitempty"`
	// Metrics is nil until the agent first reports; afterwards it holds the
	// last reported values (as of LastSeen at the latest).
	Metrics *HostMetrics `json:"metrics,omitempty"`
	// InstalledBy is the user who provisioned the agent. Used for visibility of
	// an unbound agent; not surfaced to the dashboard.
	InstalledBy string `json:"-"`
}

// Thresholds defines the age cutoffs (now − last_seen) used to derive
// ConnectionStatus. Must satisfy StaleAfter ≤ DisconnectedAfter ≤ LostAfter.
type Thresholds struct {
	StaleAfter        time.Duration `mapstructure:"stale_after"`
	DisconnectedAfter time.Duration `mapstructure:"disconnected_after"`
	LostAfter         time.Duration `mapstructure:"lost_after"`
}

// Validate enforces StaleAfter ≤ DisconnectedAfter ≤ LostAfter and rejects
// non-positive values.
func (t Thresholds) Validate() error {
	if t.StaleAfter <= 0 || t.DisconnectedAfter <= 0 || t.LostAfter <= 0 {
		return errors.New("agentregistry thresholds must be positive durations")
	}
	if t.StaleAfter > t.DisconnectedAfter || t.DisconnectedAfter > t.LostAfter {
		return errors.New("agentregistry thresholds must satisfy stale_after ≤ disconnected_after ≤ lost_after")
	}
	return nil
}

// Service manages agent registration and heartbeats backed by PostgreSQL.
type Service struct {
	pool       *pgxpool.Pool
	q          *sqlc.Queries
	thresholds Thresholds
}

func NewService(pool *pgxpool.Pool, q *sqlc.Queries, thresholds Thresholds) *Service {
	return &Service{
		pool:       pool,
		q:          q,
		thresholds: thresholds,
	}
}

// MintToken returns a fresh token without persisting an agent. The install flow
// shows the command first, then hands this same token to Create on confirmation.
func (s *Service) MintToken() (rawToken string, err error) {
	raw, _, err := generateToken()
	return raw, err
}

// Create pre-provisions an agent owned (for visibility) by installedByUserID.
// A non-empty presetToken (from the two-step install flow) has its digest
// stored; otherwise a fresh token is generated. The raw token is returned once
// and never stored. Returns ErrNameTaken on duplicate name.
func (s *Service) Create(ctx context.Context, name, installedByUserID, presetToken string) (agentID, rawToken string, err error) {
	uid := pgtype.UUID{}
	if err = uid.Scan(installedByUserID); err != nil {
		return "", "", err
	}
	var hash string
	if presetToken != "" {
		if !validToken(presetToken) {
			return "", "", ErrBadToken
		}
		rawToken, hash = presetToken, hashToken(presetToken)
	} else {
		rawToken, hash, err = generateToken()
		if err != nil {
			return "", "", err
		}
	}
	a, err := s.q.CreateAgent(ctx, sqlc.CreateAgentParams{
		Name:              name,
		InstalledByUserID: uid,
		TokenHash:         pgtype.Text{String: hash, Valid: true},
	})
	if err != nil {
		if isUniqueViolation(err) {
			return "", "", ErrNameTaken
		}
		return "", "", err
	}
	return util.UUIDToString(a.ID), rawToken, nil
}

// ReissueToken mints a fresh token for an existing agent, replacing any prior
// digest. Used for migration of pre-token agents and token-loss recovery.
func (s *Service) ReissueToken(ctx context.Context, agentID string) (rawToken string, err error) {
	aid := pgtype.UUID{}
	if err = aid.Scan(agentID); err != nil {
		return "", err
	}
	raw, hash, err := generateToken()
	if err != nil {
		return "", err
	}
	if _, err = s.q.SetAgentToken(ctx, sqlc.SetAgentTokenParams{
		ID:        aid,
		TokenHash: pgtype.Text{String: hash, Valid: true},
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return raw, nil
}

// ResolveByToken hashes the presented token and resolves the owning agent.
// Returns ErrNotFound when no agent carries that digest. project_name is not
// populated by this path.
func (s *Service) ResolveByToken(ctx context.Context, rawToken string) (*AgentStatus, error) {
	a, err := s.q.GetAgentByTokenHash(ctx, pgtype.Text{String: hashToken(rawToken), Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	status := s.toStatus(a, "", time.Now())
	return &status, nil
}

// Touch records a heartbeat on an already-resolved agent. Empty agentType or
// version leaves any existing stored value intact (see TouchAgent SQL); a nil
// metrics likewise keeps the last reported snapshot.
func (s *Service) Touch(ctx context.Context, agentID, remoteIP, agentType, version string, metrics *HostMetrics) error {
	aid := pgtype.UUID{}
	if err := aid.Scan(agentID); err != nil {
		return err
	}
	params := sqlc.TouchAgentParams{
		ID:        aid,
		RemoteIp:  remoteIP,
		AgentType: agentType,
		Version:   version,
	}
	if metrics != nil {
		params.CpuPercent = pgtype.Float4{Float32: float32(metrics.CPUPercent), Valid: true}
		params.MemUsedBytes = pgtype.Int8{Int64: int64(metrics.MemUsedBytes), Valid: true}
		params.MemTotalBytes = pgtype.Int8{Int64: int64(metrics.MemTotalBytes), Valid: true}
		params.SwapUsedBytes = pgtype.Int8{Int64: int64(metrics.SwapUsedBytes), Valid: true}
		params.SwapTotalBytes = pgtype.Int8{Int64: int64(metrics.SwapTotalBytes), Valid: true}
		params.DiskUsedBytes = pgtype.Int8{Int64: int64(metrics.DiskUsedBytes), Valid: true}
		params.DiskTotalBytes = pgtype.Int8{Int64: int64(metrics.DiskTotalBytes), Valid: true}
	}
	return s.q.TouchAgent(ctx, params)
}

// ListForUser returns the agents visible to userID: unbound agents they
// installed, plus agents bound to projects they're a member of.
func (s *Service) ListForUser(ctx context.Context, userID string) ([]AgentStatus, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListAgentsForUser(ctx, uid)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	out := make([]AgentStatus, len(rows))
	for i, r := range rows {
		out[i] = s.toStatus(agentFromListRow(r), r.ProjectName.String, now)
	}
	return out, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*AgentStatus, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return nil, err
	}
	r, err := s.q.GetAgent(ctx, uid)
	if err != nil {
		return nil, err
	}
	status := s.toStatus(agentFromGetRow(r), r.ProjectName.String, time.Now())
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
		if _, err := s.q.BindAgentToProject(ctx, sqlc.BindAgentToProjectParams{
			ID:        aid,
			ProjectID: pid,
		}); err != nil {
			return nil, err
		}
		return s.GetByID(ctx, agentID)
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

	if _, err = q.BindAgentToProject(ctx, sqlc.BindAgentToProjectParams{
		ID:        aid,
		ProjectID: pid,
	}); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.GetByID(ctx, agentID)
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

func (s *Service) toStatus(a sqlc.Agent, projectName string, now time.Time) AgentStatus {
	// No heartbeat yet: a freshly provisioned agent has never connected.
	var lastSeen *time.Time
	status := StatusDisconnected
	if a.LastSeenAt.Valid {
		t := a.LastSeenAt.Time
		lastSeen = &t
		switch age := now.Sub(t); {
		case age < s.thresholds.StaleAfter:
			status = StatusConnected
		case age < s.thresholds.DisconnectedAfter:
			status = StatusStale
		case age < s.thresholds.LostAfter:
			status = StatusDisconnected
		default:
			status = StatusLost
		}
	}
	return AgentStatus{
		ID:          util.UUIDToString(a.ID),
		Name:        a.Name,
		LastSeen:    lastSeen,
		RemoteIP:    a.RemoteIp,
		Status:      status,
		ProjectID:   util.UUIDToString(a.ProjectID),
		ProjectName: projectName,
		Type:        a.AgentType.String,
		Version:     a.Version.String,
		Metrics:     metricsFromAgent(a),
		InstalledBy: util.UUIDToString(a.InstalledByUserID),
	}
}

// metricsFromAgent returns nil for agents that have never reported metrics.
// The metric columns are always written together, so mem_total stands in for
// all of them.
func metricsFromAgent(a sqlc.Agent) *HostMetrics {
	if !a.MemTotalBytes.Valid {
		return nil
	}
	return &HostMetrics{
		CPUPercent:     float64(a.CpuPercent.Float32),
		MemUsedBytes:   uint64(a.MemUsedBytes.Int64),
		MemTotalBytes:  uint64(a.MemTotalBytes.Int64),
		SwapUsedBytes:  uint64(a.SwapUsedBytes.Int64),
		SwapTotalBytes: uint64(a.SwapTotalBytes.Int64),
		DiskUsedBytes:  uint64(a.DiskUsedBytes.Int64),
		DiskTotalBytes: uint64(a.DiskTotalBytes.Int64),
	}
}

func agentFromListRow(r sqlc.ListAgentsForUserRow) sqlc.Agent {
	return sqlc.Agent{
		ID:                r.ID,
		Name:              r.Name,
		RemoteIp:          r.RemoteIp,
		LastSeenAt:        r.LastSeenAt,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
		ProjectID:         r.ProjectID,
		AgentType:         r.AgentType,
		Version:           r.Version,
		InstalledByUserID: r.InstalledByUserID,
		CpuPercent:        r.CpuPercent,
		MemUsedBytes:      r.MemUsedBytes,
		MemTotalBytes:     r.MemTotalBytes,
		SwapUsedBytes:     r.SwapUsedBytes,
		SwapTotalBytes:    r.SwapTotalBytes,
		DiskUsedBytes:     r.DiskUsedBytes,
		DiskTotalBytes:    r.DiskTotalBytes,
	}
}

func agentFromGetRow(r sqlc.GetAgentRow) sqlc.Agent {
	return sqlc.Agent{
		ID:                r.ID,
		Name:              r.Name,
		RemoteIp:          r.RemoteIp,
		LastSeenAt:        r.LastSeenAt,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
		ProjectID:         r.ProjectID,
		AgentType:         r.AgentType,
		Version:           r.Version,
		InstalledByUserID: r.InstalledByUserID,
		CpuPercent:        r.CpuPercent,
		MemUsedBytes:      r.MemUsedBytes,
		MemTotalBytes:     r.MemTotalBytes,
		SwapUsedBytes:     r.SwapUsedBytes,
		SwapTotalBytes:    r.SwapTotalBytes,
		DiskUsedBytes:     r.DiskUsedBytes,
		DiskTotalBytes:    r.DiskTotalBytes,
	}
}

// isUniqueViolation reports whether err is a Postgres unique-constraint error
// (SQLSTATE 23505), e.g. a duplicate agent name.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
