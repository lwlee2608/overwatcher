package project

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
	ErrNotFound        = errors.New("project not found")
	ErrServiceNotFound = errors.New("service not found")
	ErrUserNotFound    = errors.New("user not found")
)

// Project is a deployable unit owned by a user. It owns a single compose
// file and binds 1:1 to an agent.
type Project struct {
	ID          string
	UserID      string
	UserEmail   string
	Name        string
	Description string
	ComposeFile string
	Environment string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ComposeService is a row in the services table — one compose-file service
// inside a project, linked to a GitHub repo + root directory.
type ComposeService struct {
	ID            string
	ProjectID     string
	Name          string
	Repo          string
	RootDirectory string
	Branch        string
	Image         string
	Tag           string
	Position      int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Service struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: sqlc.New(pool)}
}

// --- Project CRUD ---

type CreateProjectParams struct {
	UserID      string
	Name        string
	Description string
	ComposeFile string
	Environment string
	Enabled     bool
}

func (s *Service) CreateProject(ctx context.Context, p CreateProjectParams) (*Project, error) {
	userUID := pgtype.UUID{}
	if err := userUID.Scan(p.UserID); err != nil {
		return nil, err
	}
	env := p.Environment
	if env == "" {
		env = "production"
	}
	row, err := s.q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID:      userUID,
		Name:        p.Name,
		Description: p.Description,
		ComposeFile: p.ComposeFile,
		Environment: env,
		Enabled:     p.Enabled,
	})
	if err != nil {
		return nil, err
	}
	return s.GetProject(ctx, util.UUIDToString(row.ID))
}

func (s *Service) GetProject(ctx context.Context, id string) (*Project, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return nil, err
	}
	row, err := s.q.GetProject(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	// Resolve email via a second query to keep GetProject returning a bare
	// row from the generated code. Cheap — single row lookup.
	user, err := s.q.GetUser(ctx, row.UserID)
	if err != nil {
		return nil, err
	}
	return projectRowToDomain(row, user.Email), nil
}

func (s *Service) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.q.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Project, len(rows))
	for i, r := range rows {
		out[i] = Project{
			ID:          util.UUIDToString(r.ID),
			UserID:      util.UUIDToString(r.UserID),
			UserEmail:   r.UserEmail,
			Name:        r.Name,
			Description: r.Description,
			ComposeFile: r.ComposeFile,
			Environment: r.Environment,
			Enabled:     r.Enabled,
			CreatedAt:   r.CreatedAt.Time,
			UpdatedAt:   r.UpdatedAt.Time,
		}
	}
	return out, nil
}

func (s *Service) ListProjectsByUser(ctx context.Context, userID string) ([]Project, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListProjectsByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]Project, len(rows))
	for i, r := range rows {
		out[i] = *projectRowToDomain(r, "")
	}
	return out, nil
}

type UpdateProjectParams struct {
	Name        string
	Description string
	ComposeFile string
	Environment string
	Enabled     bool
}

func (s *Service) UpdateProject(ctx context.Context, id string, p UpdateProjectParams) (*Project, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return nil, err
	}
	env := p.Environment
	if env == "" {
		env = "production"
	}
	_, err := s.q.UpdateProject(ctx, sqlc.UpdateProjectParams{
		ID:          uid,
		Name:        p.Name,
		Description: p.Description,
		ComposeFile: p.ComposeFile,
		Environment: env,
		Enabled:     p.Enabled,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.GetProject(ctx, id)
}

func (s *Service) DeleteProject(ctx context.Context, id string) error {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return err
	}
	if _, err := s.q.DeleteProject(ctx, uid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// --- ComposeService CRUD ---

type CreateComposeServiceParams struct {
	ProjectID     string
	Name          string
	Repo          string
	RootDirectory string
	Branch        string
	Image         string
	Tag           string
	Position      int
}

func (s *Service) CreateComposeService(ctx context.Context, p CreateComposeServiceParams) (*ComposeService, error) {
	projUID := pgtype.UUID{}
	if err := projUID.Scan(p.ProjectID); err != nil {
		return nil, err
	}
	root := p.RootDirectory
	if root == "" {
		root = "/"
	}
	branch := p.Branch
	if branch == "" {
		branch = "main"
	}
	tag := p.Tag
	if tag == "" {
		tag = "latest"
	}
	row, err := s.q.CreateService(ctx, sqlc.CreateServiceParams{
		ProjectID:     projUID,
		Name:          p.Name,
		Repo:          p.Repo,
		RootDirectory: root,
		Branch:        branch,
		Image:         p.Image,
		Tag:           tag,
		Position:      int32(p.Position),
	})
	if err != nil {
		return nil, err
	}
	return composeRowToDomain(row), nil
}

func (s *Service) GetComposeService(ctx context.Context, id string) (*ComposeService, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return nil, err
	}
	row, err := s.q.GetService(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return composeRowToDomain(row), nil
}

func (s *Service) ListComposeServicesByProject(ctx context.Context, projectID string) ([]ComposeService, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(projectID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListServicesByProject(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]ComposeService, len(rows))
	for i, r := range rows {
		out[i] = *composeRowToDomain(r)
	}
	return out, nil
}

func (s *Service) DeleteComposeService(ctx context.Context, id string) error {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return err
	}
	if _, err := s.q.DeleteService(ctx, uid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrServiceNotFound
		}
		return err
	}
	return nil
}

// RepoMatch is one row returned by ListEnabledServicesByRepoAndBranch — a
// compose service joined with its owning project. The webhook handler groups
// these by ProjectID to produce one intent per project.
type RepoMatch struct {
	ServiceID          string
	ProjectID          string
	ProjectName        string
	ProjectUserID      string
	ProjectComposeFile string
	ProjectEnvironment string
	ServiceName        string
	Repo               string
	RootDirectory      string
	Branch             string
	Image              string
	Tag                string
	Position           int
}

// ListEnabledServicesByRepoAndBranch returns every enabled service whose repo
// and branch match. Callers still have to filter by root_directory against
// the pushed changed paths before enqueueing.
func (s *Service) ListEnabledServicesByRepoAndBranch(ctx context.Context, repo, branch string) ([]RepoMatch, error) {
	rows, err := s.q.ListEnabledServicesByRepoAndBranch(ctx, sqlc.ListEnabledServicesByRepoAndBranchParams{
		Lower:  repo,
		Branch: branch,
	})
	if err != nil {
		return nil, err
	}
	out := make([]RepoMatch, len(rows))
	for i, r := range rows {
		out[i] = RepoMatch{
			ServiceID:          util.UUIDToString(r.ID),
			ProjectID:          util.UUIDToString(r.ProjectID),
			ProjectName:        r.ProjectName,
			ProjectUserID:      util.UUIDToString(r.ProjectUserID),
			ProjectComposeFile: r.ProjectComposeFile,
			ProjectEnvironment: r.ProjectEnvironment,
			ServiceName:        r.Name,
			Repo:               r.Repo,
			RootDirectory:      r.RootDirectory,
			Branch:             r.Branch,
			Image:              r.Image,
			Tag:                r.Tag,
			Position:           int(r.Position),
		}
	}
	return out, nil
}

// ReplaceComposeServices rewrites the services list for a project
// transactionally — used by the "edit project" UI path.
func (s *Service) ReplaceComposeServices(ctx context.Context, projectID string, services []CreateComposeServiceParams) ([]ComposeService, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(projectID); err != nil {
		return nil, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.DeleteServicesByProject(ctx, uid); err != nil {
		return nil, err
	}
	out := make([]ComposeService, len(services))
	for i, p := range services {
		root := p.RootDirectory
		if root == "" {
			root = "/"
		}
		branch := p.Branch
		if branch == "" {
			branch = "main"
		}
		tag := p.Tag
		if tag == "" {
			tag = "latest"
		}
		row, err := q.CreateService(ctx, sqlc.CreateServiceParams{
			ProjectID:     uid,
			Name:          p.Name,
			Repo:          p.Repo,
			RootDirectory: root,
			Branch:        branch,
			Image:         p.Image,
			Tag:           tag,
			Position:      int32(i),
		})
		if err != nil {
			return nil, err
		}
		out[i] = *composeRowToDomain(row)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

func projectRowToDomain(r sqlc.Project, email string) *Project {
	return &Project{
		ID:          util.UUIDToString(r.ID),
		UserID:      util.UUIDToString(r.UserID),
		UserEmail:   email,
		Name:        r.Name,
		Description: r.Description,
		ComposeFile: r.ComposeFile,
		Environment: r.Environment,
		Enabled:     r.Enabled,
		CreatedAt:   r.CreatedAt.Time,
		UpdatedAt:   r.UpdatedAt.Time,
	}
}

func composeRowToDomain(r sqlc.Service) *ComposeService {
	return &ComposeService{
		ID:            util.UUIDToString(r.ID),
		ProjectID:     util.UUIDToString(r.ProjectID),
		Name:          r.Name,
		Repo:          r.Repo,
		RootDirectory: r.RootDirectory,
		Branch:        r.Branch,
		Image:         r.Image,
		Tag:           r.Tag,
		Position:      int(r.Position),
		CreatedAt:     r.CreatedAt.Time,
		UpdatedAt:     r.UpdatedAt.Time,
	}
}
