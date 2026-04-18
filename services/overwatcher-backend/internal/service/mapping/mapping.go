package mapping

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
	"github.com/lwlee2608/overwatcher/internal/util"
)

// ErrNotFound is returned when a mapping does not exist.
var ErrNotFound = errors.New("mapping not found")

// ErrAgentNotFound is returned when the referenced agent does not exist.
var ErrAgentNotFound = errors.New("agent not found")

// ServiceSpec pairs a compose service name with its image and tag. An empty
// Name means "apply to every service in the compose file".
type ServiceSpec struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Tag   string `json:"tag"`
}

// Entry is a resolved mapping row used by the webhook handler.
type Entry struct {
	ID          string
	Repo        string
	AgentID     string
	AgentName   string
	Services    []ServiceSpec
	Environment string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Service manages deploy mappings backed by PostgreSQL.
type Service struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: sqlc.New(pool)}
}

// Match returns all enabled mappings for the given repo (case-insensitive).
func (s *Service) Match(ctx context.Context, repo string) ([]Entry, error) {
	rows, err := s.q.ListEnabledMappingsByRepo(ctx, repo)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, len(rows))
	for i, r := range rows {
		services, err := decodeServices(r.Services)
		if err != nil {
			return nil, err
		}
		entries[i] = Entry{
			ID:          util.UUIDToString(r.ID),
			Repo:        r.Repo,
			AgentID:     util.UUIDToString(r.AgentID),
			AgentName:   r.AgentName,
			Services:    services,
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
		services, err := decodeServices(r.Services)
		if err != nil {
			return nil, err
		}
		entries[i] = Entry{
			ID:          util.UUIDToString(r.ID),
			Repo:        r.Repo,
			AgentID:     util.UUIDToString(r.AgentID),
			AgentName:   r.AgentName,
			Services:    services,
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
	services, err := decodeServices(r.Services)
	if err != nil {
		return nil, err
	}
	return &Entry{
		ID:          util.UUIDToString(r.ID),
		Repo:        r.Repo,
		AgentID:     util.UUIDToString(r.AgentID),
		AgentName:   r.AgentName,
		Services:    services,
		Environment: r.Environment,
		Enabled:     r.Enabled,
		CreatedAt:   r.CreatedAt.Time,
		UpdatedAt:   r.UpdatedAt.Time,
	}, nil
}

type CreateParams struct {
	Repo        string
	AgentID     string
	Services    []ServiceSpec
	Environment string
	Enabled     bool
}

// Create inserts a new mapping plus its service rows in a single transaction.
// Returns ErrAgentNotFound if the agent_id doesn't exist.
func (s *Service) Create(ctx context.Context, p CreateParams) (*Entry, error) {
	agentUID := pgtype.UUID{}
	if err := agentUID.Scan(p.AgentID); err != nil {
		return nil, err
	}
	env := p.Environment
	if env == "" {
		env = "production"
	}

	var id pgtype.UUID
	err := s.withTx(ctx, func(q *sqlc.Queries) error {
		if _, err := q.GetAgent(ctx, agentUID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAgentNotFound
			}
			return err
		}
		row, err := q.CreateDeployMapping(ctx, sqlc.CreateDeployMappingParams{
			Repo:        p.Repo,
			AgentID:     agentUID,
			Environment: env,
			Enabled:     p.Enabled,
		})
		if err != nil {
			return err
		}
		id = row.ID
		return insertServices(ctx, q, row.ID, p.Services)
	})
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, util.UUIDToString(id))
}

type UpdateParams struct {
	Repo        string
	AgentID     string
	Services    []ServiceSpec
	Environment string
	Enabled     bool
}

// Update modifies an existing mapping. Service rows are fully rewritten.
// Returns ErrAgentNotFound if the agent_id doesn't exist.
func (s *Service) Update(ctx context.Context, id string, p UpdateParams) (*Entry, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return nil, err
	}
	agentUID := pgtype.UUID{}
	if err := agentUID.Scan(p.AgentID); err != nil {
		return nil, err
	}
	env := p.Environment
	if env == "" {
		env = "production"
	}

	err := s.withTx(ctx, func(q *sqlc.Queries) error {
		if _, err := q.GetAgent(ctx, agentUID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrAgentNotFound
			}
			return err
		}
		if _, err := q.UpdateDeployMapping(ctx, sqlc.UpdateDeployMappingParams{
			ID:          uid,
			Repo:        p.Repo,
			AgentID:     agentUID,
			Environment: env,
			Enabled:     p.Enabled,
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if err := q.DeleteMappingServicesByMapping(ctx, uid); err != nil {
			return err
		}
		return insertServices(ctx, q, uid, p.Services)
	})
	if err != nil {
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
	_, err := s.q.DeleteDeployMapping(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) withTx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertServices(ctx context.Context, q *sqlc.Queries, mappingID pgtype.UUID, services []ServiceSpec) error {
	for i, svc := range services {
		tag := svc.Tag
		if tag == "" {
			tag = "latest"
		}
		if err := q.CreateMappingService(ctx, sqlc.CreateMappingServiceParams{
			MappingID: mappingID,
			Name:      svc.Name,
			Image:     svc.Image,
			Tag:       tag,
			Position:  int32(i),
		}); err != nil {
			return err
		}
	}
	return nil
}

// decodeServices converts the jsonb_agg result into a typed slice. pgx v5
// decodes JSONB into generic Go values ([]interface{}, map[string]interface{})
// when scanned into interface{}, so we round-trip through JSON.
func decodeServices(raw interface{}) ([]ServiceSpec, error) {
	if raw == nil {
		return nil, nil
	}
	var b []byte
	switch v := raw.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		marshaled, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("services column marshal (%T): %w", raw, err)
		}
		b = marshaled
	}
	if len(b) == 0 {
		return nil, nil
	}
	var out []ServiceSpec
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("services column unmarshal: %w", err)
	}
	return out, nil
}
