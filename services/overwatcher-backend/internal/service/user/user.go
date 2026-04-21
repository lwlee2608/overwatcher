package user

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
	ErrNotFound      = errors.New("user not found")
	ErrEmailConflict = errors.New("email already in use")
)

type Entry struct {
	ID        string
	Email     string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Service struct {
	q *sqlc.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: sqlc.New(pool)}
}

type CreateParams struct {
	Email string
	Name  string
}

func (s *Service) Create(ctx context.Context, p CreateParams) (*Entry, error) {
	row, err := s.q.CreateUser(ctx, sqlc.CreateUserParams{
		Email: p.Email,
		Name:  p.Name,
	})
	if err != nil {
		return nil, err
	}
	return rowToEntry(row), nil
}

func (s *Service) GetByID(ctx context.Context, id string) (*Entry, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return nil, err
	}
	row, err := s.q.GetUser(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return rowToEntry(row), nil
}

func (s *Service) GetByEmail(ctx context.Context, email string) (*Entry, error) {
	row, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return rowToEntry(row), nil
}

func (s *Service) List(ctx context.Context) ([]Entry, error) {
	rows, err := s.q.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, len(rows))
	for i, r := range rows {
		out[i] = *rowToEntry(r)
	}
	return out, nil
}

type UpdateParams struct {
	Email string
	Name  string
}

func (s *Service) Update(ctx context.Context, id string, p UpdateParams) (*Entry, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return nil, err
	}
	row, err := s.q.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:    uid,
		Email: p.Email,
		Name:  p.Name,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return rowToEntry(row), nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	uid := pgtype.UUID{}
	if err := uid.Scan(id); err != nil {
		return err
	}
	if _, err := s.q.DeleteUser(ctx, uid); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func rowToEntry(r sqlc.User) *Entry {
	return &Entry{
		ID:        util.UUIDToString(r.ID),
		Email:     r.Email,
		Name:      r.Name,
		CreatedAt: r.CreatedAt.Time,
		UpdatedAt: r.UpdatedAt.Time,
	}
}
