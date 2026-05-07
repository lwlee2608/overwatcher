package project

import (
	"context"
	"errors"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
	"github.com/lwlee2608/overwatcher/internal/util"
)

// normalizeWorkflow stores workflows by filename only. Users sometimes paste
// the full path (".github/workflows/build.yml") because that's how GitHub
// displays it; the workflow_run handler matches on path.Base(), so without
// this normalization those services would silently never deploy.
func normalizeWorkflow(w string) string {
	w = strings.TrimSpace(w)
	if w == "" {
		return ""
	}
	return path.Base(w)
}

var repoSlugRe = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

func normalizeRepo(r string) (string, error) {
	r = strings.TrimSpace(r)
	r = strings.TrimPrefix(r, "https://github.com/")
	r = strings.TrimPrefix(r, "http://github.com/")
	r = strings.TrimPrefix(r, "git@github.com:")
	r = strings.TrimSuffix(r, "/")
	r = strings.TrimSuffix(r, ".git")
	if !repoSlugRe.MatchString(r) {
		return "", ErrInvalidRepo
	}
	return r, nil
}

var (
	ErrNotFound        = errors.New("project not found")
	ErrServiceNotFound = errors.New("service not found")
	ErrUserNotFound    = errors.New("user not found")
	ErrMemberNotFound  = errors.New("member not found")
	ErrAlreadyMember   = errors.New("user is already a member")
	ErrCannotAddOwner  = errors.New("project owner cannot be a member")
	ErrForbidden       = errors.New("forbidden")
	ErrInvalidRepo     = errors.New("repo must be in 'owner/repo' format")
)

// Roles returned by Access. Owner = projects.user_id; Member = row in
// project_members. Members can view and trigger deploys; owner-only actions
// are gated by the handler layer.
const (
	RoleOwner  = "owner"
	RoleMember = "member"
)

// Member is a row of project_members joined with the user's identity.
type Member struct {
	ProjectID string
	UserID    string
	UserEmail string
	UserName  string
	Role      string
	AddedBy   string
	CreatedAt time.Time
}

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
	// Role is the caller's role for this project ("owner" or "member"),
	// populated only by listings that take a viewer userID. Empty otherwise.
	Role        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ComposeService is a row in the services table — one compose-file service
// inside a project, linked to a GitHub repo + root directory.
//
// Workflow, if non-empty, names the GitHub Actions workflow file (e.g.
// "build-and-publish.yml") whose successful completion triggers a deploy.
// Empty means deploy on push (legacy behavior).
type ComposeService struct {
	ID            string
	ProjectID     string
	Name          string
	Repo          string
	RootDirectory string
	Branch        string
	Image         string
	Tag           string
	Workflow      string
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
	Workflow      string
	Position      int
}

func (s *Service) CreateComposeService(ctx context.Context, p CreateComposeServiceParams) (*ComposeService, error) {
	projUID := pgtype.UUID{}
	if err := projUID.Scan(p.ProjectID); err != nil {
		return nil, err
	}
	repo, err := normalizeRepo(p.Repo)
	if err != nil {
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
		Repo:          repo,
		RootDirectory: root,
		Branch:        branch,
		Image:         p.Image,
		Tag:           tag,
		Workflow:      normalizeWorkflow(p.Workflow),
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

// DeleteComposeService removes a service only if it belongs to the given
// project. The project_id check defends against cross-project IDOR — without
// it, a member of project A could delete services from project B by guessing
// service UUIDs.
func (s *Service) DeleteComposeService(ctx context.Context, projectID, serviceID string) error {
	pUID := pgtype.UUID{}
	if err := pUID.Scan(projectID); err != nil {
		return err
	}
	sUID := pgtype.UUID{}
	if err := sUID.Scan(serviceID); err != nil {
		return err
	}
	if _, err := s.q.DeleteServiceInProject(ctx, sqlc.DeleteServiceInProjectParams{ID: sUID, ProjectID: pUID}); err != nil {
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
	Workflow           string
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
			Workflow:           r.Workflow,
			Position:           int(r.Position),
		}
	}
	return out, nil
}

// ListEnabledServicesByRepoAndWorkflow returns every enabled service whose
// repo+branch+workflow matches. Used by the workflow_run webhook handler.
func (s *Service) ListEnabledServicesByRepoAndWorkflow(ctx context.Context, repo, branch, workflow string) ([]RepoMatch, error) {
	rows, err := s.q.ListEnabledServicesByRepoAndWorkflow(ctx, sqlc.ListEnabledServicesByRepoAndWorkflowParams{
		Lower:    repo,
		Branch:   branch,
		Workflow: workflow,
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
			Workflow:           r.Workflow,
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
	// Validate all rows before DeleteServicesByProject so a typo can't leave
	// the project with zero services.
	repos := make([]string, len(services))
	for i, p := range services {
		repo, err := normalizeRepo(p.Repo)
		if err != nil {
			return nil, err
		}
		repos[i] = repo
	}
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
			Repo:          repos[i],
			RootDirectory: root,
			Branch:        branch,
			Image:         p.Image,
			Tag:           tag,
			Workflow:      normalizeWorkflow(p.Workflow),
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

// Access returns the caller's role on a project. Returns ErrNotFound when the
// project does not exist; ErrForbidden when the user has neither ownership
// nor a project_members row.
func (s *Service) Access(ctx context.Context, userID, projectID string) (string, error) {
	pUID := pgtype.UUID{}
	if err := pUID.Scan(projectID); err != nil {
		return "", err
	}
	uUID := pgtype.UUID{}
	if err := uUID.Scan(userID); err != nil {
		return "", err
	}
	row, err := s.q.GetProject(ctx, pUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if util.UUIDToString(row.UserID) == userID {
		return RoleOwner, nil
	}
	m, err := s.q.GetProjectMember(ctx, sqlc.GetProjectMemberParams{ProjectID: pUID, UserID: uUID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrForbidden
		}
		return "", err
	}
	return m.Role, nil
}

// ListProjectsForUser returns owned + shared projects for a user, with the
// per-row Role populated.
func (s *Service) ListProjectsForUser(ctx context.Context, userID string) ([]Project, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListProjectsForUser(ctx, uid)
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
			Role:        r.Role,
			CreatedAt:   r.CreatedAt.Time,
			UpdatedAt:   r.UpdatedAt.Time,
		}
	}
	return out, nil
}

// AddMember inserts a project_members row. Caller must be the owner; the
// handler enforces that. Returns ErrCannotAddOwner if userID matches the
// project's owner, ErrAlreadyMember on duplicate.
func (s *Service) AddMember(ctx context.Context, projectID, userID, addedBy string) (*Member, error) {
	pUID := pgtype.UUID{}
	if err := pUID.Scan(projectID); err != nil {
		return nil, err
	}
	uUID := pgtype.UUID{}
	if err := uUID.Scan(userID); err != nil {
		return nil, err
	}
	addedByUID := pgtype.UUID{}
	if addedBy != "" {
		if err := addedByUID.Scan(addedBy); err != nil {
			return nil, err
		}
	}
	proj, err := s.q.GetProject(ctx, pUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if util.UUIDToString(proj.UserID) == userID {
		return nil, ErrCannotAddOwner
	}
	user, err := s.q.GetUser(ctx, uUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	row, err := s.q.AddProjectMember(ctx, sqlc.AddProjectMemberParams{
		ProjectID: pUID,
		UserID:    uUID,
		Role:      RoleMember,
		AddedBy:   addedByUID,
	})
	if err != nil {
		var pgErr interface{ SQLState() string }
		if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
			return nil, ErrAlreadyMember
		}
		return nil, err
	}
	return &Member{
		ProjectID: util.UUIDToString(row.ProjectID),
		UserID:    util.UUIDToString(row.UserID),
		UserEmail: user.Email,
		UserName:  user.Name,
		Role:      row.Role,
		AddedBy:   util.UUIDToString(row.AddedBy),
		CreatedAt: row.CreatedAt.Time,
	}, nil
}

// AddMemberByEmail resolves the email to an existing user, then delegates to
// AddMember. Returns ErrUserNotFound when no user with that email exists.
func (s *Service) AddMemberByEmail(ctx context.Context, projectID, email, addedBy string) (*Member, error) {
	row, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return s.AddMember(ctx, projectID, util.UUIDToString(row.ID), addedBy)
}

func (s *Service) RemoveMember(ctx context.Context, projectID, userID string) error {
	pUID := pgtype.UUID{}
	if err := pUID.Scan(projectID); err != nil {
		return err
	}
	uUID := pgtype.UUID{}
	if err := uUID.Scan(userID); err != nil {
		return err
	}
	if _, err := s.q.RemoveProjectMember(ctx, sqlc.RemoveProjectMemberParams{ProjectID: pUID, UserID: uUID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMemberNotFound
		}
		return err
	}
	return nil
}

func (s *Service) ListMembers(ctx context.Context, projectID string) ([]Member, error) {
	pUID := pgtype.UUID{}
	if err := pUID.Scan(projectID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListProjectMembers(ctx, pUID)
	if err != nil {
		return nil, err
	}
	out := make([]Member, len(rows))
	for i, r := range rows {
		out[i] = Member{
			ProjectID: util.UUIDToString(r.ProjectID),
			UserID:    util.UUIDToString(r.UserID),
			UserEmail: r.UserEmail,
			UserName:  r.UserName,
			Role:      r.Role,
			AddedBy:   util.UUIDToString(r.AddedBy),
			CreatedAt: r.CreatedAt.Time,
		}
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
		Workflow:      r.Workflow,
		Position:      int(r.Position),
		CreatedAt:     r.CreatedAt.Time,
		UpdatedAt:     r.UpdatedAt.Time,
	}
}
