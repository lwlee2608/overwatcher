package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
	"github.com/lwlee2608/overwatcher/internal/util"
)

// TokenPrefix marks a user API key so it's greppable in logs and secret
// scanners, and distinguishable from agent tokens (owa_).
const TokenPrefix = "owk_"

var (
	ErrNotFound    = errors.New("api key not found")
	ErrNameTaken   = errors.New("api key name already in use")
	ErrNameMissing = errors.New("api key name is required")
)

type APIKey struct {
	ID         string
	UserID     string
	Name       string
	LastUsedAt *time.Time
	CreatedAt  time.Time
}

type Service struct {
	q *sqlc.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: sqlc.New(pool)}
}

// Create mints a key for userID. The raw key is returned once; only its
// sha256 digest is stored.
func (s *Service) Create(ctx context.Context, userID, name string) (*APIKey, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", ErrNameMissing
	}
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, "", err
	}
	raw, err := generateToken()
	if err != nil {
		return nil, "", err
	}
	row, err := s.q.CreateAPIKey(ctx, sqlc.CreateAPIKeyParams{
		UserID:    uid,
		Name:      name,
		TokenHash: hashToken(raw),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, "", ErrNameTaken
		}
		return nil, "", err
	}
	return rowToDomain(row), raw, nil
}

func (s *Service) List(ctx context.Context, userID string) ([]APIKey, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListAPIKeysByUser(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]APIKey, len(rows))
	for i, r := range rows {
		out[i] = *rowToDomain(r)
	}
	return out, nil
}

// Delete revokes a key only if it belongs to userID, so one user can't
// revoke another's key by guessing its ID.
func (s *Service) Delete(ctx context.Context, userID, id string) error {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return err
	}
	kid := pgtype.UUID{}
	if err := kid.Scan(id); err != nil {
		return ErrNotFound
	}
	if _, err := s.q.DeleteAPIKeyForUser(ctx, sqlc.DeleteAPIKeyForUserParams{ID: kid, UserID: uid}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// Resolve returns the user a raw key acts as, and records its use.
func (s *Service) Resolve(ctx context.Context, raw string) (userID string, err error) {
	if !strings.HasPrefix(raw, TokenPrefix) {
		return "", ErrNotFound
	}
	row, err := s.q.GetAPIKeyByTokenHash(ctx, hashToken(raw))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if err := s.q.TouchAPIKey(ctx, row.ID); err != nil {
		return "", err
	}
	return util.UUIDToString(row.UserID), nil
}

func generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return TokenPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func rowToDomain(r sqlc.ApiKey) *APIKey {
	k := &APIKey{
		ID:        util.UUIDToString(r.ID),
		UserID:    util.UUIDToString(r.UserID),
		Name:      r.Name,
		CreatedAt: r.CreatedAt.Time,
	}
	if r.LastUsedAt.Valid {
		t := r.LastUsedAt.Time
		k.LastUsedAt = &t
	}
	return k
}
