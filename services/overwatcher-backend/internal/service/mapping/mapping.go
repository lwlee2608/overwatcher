package mapping

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
	"github.com/lwlee2608/overwatcher/internal/util"
)

// ErrNotFound is returned when a mapping does not exist.
var ErrNotFound = errors.New("mapping not found")

// ErrAgentNotFound is returned when the referenced agent does not exist.
var ErrAgentNotFound = errors.New("agent not found")

// Entry is a resolved mapping row used by the webhook handler.
type Entry struct {
	ID          string
	Repo        string
	AgentID     string
	AgentName   string
	Services    []string
	Environment string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ResolveImage returns the default convention ghcr.io/<lowered-repo>.
func ResolveImage(repo string) string {
	return "ghcr.io/" + strings.ToLower(repo)
}

// ResolveTag returns the commit SHA as the image tag.
func ResolveTag(sha string) string {
	return sha
}

// Service manages deploy mappings backed by PostgreSQL.
type Service struct {
	q *sqlc.Queries
}

func NewService(q *sqlc.Queries) *Service {
	return &Service{q: q}
}

// Match returns all enabled mappings for the given repo (case-insensitive).
func (s *Service) Match(ctx context.Context, repo string) ([]Entry, error) {
	rows, err := s.q.ListEnabledMappingsByRepo(ctx, repo)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, len(rows))
	for i, r := range rows {
		entries[i] = Entry{
			ID:          util.UUIDToString(r.ID),
			Repo:        r.Repo,
			AgentID:     util.UUIDToString(r.AgentID),
			AgentName:   r.AgentName,
			Services:    r.Services,
			Environment: r.Environment,
			Enabled:     r.Enabled,
			CreatedAt:   r.CreatedAt.Time,
			UpdatedAt:   r.UpdatedAt.Time,
		}
	}
	return entries, nil
}

// List returns all mappings.
func (s *Service) List(ctx context.Context) ([]Entry, error) {
	rows, err := s.q.ListDeployMappings(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, len(rows))
	for i, r := range rows {
		entries[i] = Entry{
			ID:          util.UUIDToString(r.ID),
			Repo:        r.Repo,
			AgentID:     util.UUIDToString(r.AgentID),
			AgentName:   r.AgentName,
			Services:    r.Services,
			Environment: r.Environment,
			Enabled:     r.Enabled,
			CreatedAt:   r.CreatedAt.Time,
			UpdatedAt:   r.UpdatedAt.Time,
		}
	}
	return entries, nil
}

// GetByID returns a single mapping.
func (s *Service) GetByID(ctx context.Context, id string) (*Entry, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return nil, err
	}
	r, err := s.q.GetDeployMapping(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &Entry{
		ID:          util.UUIDToString(r.ID),
		Repo:        r.Repo,
		AgentID:     util.UUIDToString(r.AgentID),
		AgentName:   r.AgentName,
		Services:    r.Services,
		Environment: r.Environment,
		Enabled:     r.Enabled,
		CreatedAt:   r.CreatedAt.Time,
		UpdatedAt:   r.UpdatedAt.Time,
	}, nil
}

type CreateParams struct {
	Repo        string
	AgentID     string
	Services    []string
	Environment string
	Enabled     bool
}

// Create inserts a new mapping. Returns ErrAgentNotFound if the agent_id doesn't exist.
func (s *Service) Create(ctx context.Context, p CreateParams) (*Entry, error) {
	agentUID := pgtype.UUID{}
	if err := agentUID.Scan(p.AgentID); err != nil {
		return nil, err
	}
	if _, err := s.q.GetAgent(ctx, agentUID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	env := p.Environment
	if env == "" {
		env = "production"
	}
	r, err := s.q.CreateDeployMapping(ctx, sqlc.CreateDeployMappingParams{
		Repo:        p.Repo,
		AgentID:     agentUID,
		Services:    p.Services,
		Environment: env,
		Enabled:     p.Enabled,
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, util.UUIDToString(r.ID))
}

type UpdateParams struct {
	Repo        string
	AgentID     string
	Services    []string
	Environment string
	Enabled     bool
}

// Update modifies an existing mapping. Returns ErrAgentNotFound if the agent_id doesn't exist.
func (s *Service) Update(ctx context.Context, id string, p UpdateParams) (*Entry, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return nil, err
	}
	agentUID := pgtype.UUID{}
	if err := agentUID.Scan(p.AgentID); err != nil {
		return nil, err
	}
	if _, err := s.q.GetAgent(ctx, agentUID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	env := p.Environment
	if env == "" {
		env = "production"
	}
	_, err := s.q.UpdateDeployMapping(ctx, sqlc.UpdateDeployMappingParams{
		ID:          uid,
		Repo:        p.Repo,
		AgentID:     agentUID,
		Services:    p.Services,
		Environment: env,
		Enabled:     p.Enabled,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// Delete removes a mapping. Returns ErrNotFound if the mapping doesn't exist.
func (s *Service) Delete(ctx context.Context, id string) error {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return err
	}
	// Check existence first since DELETE doesn't report "not found".
	if _, err := s.q.GetDeployMapping(ctx, uid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return s.q.DeleteDeployMapping(ctx, uid)
}
