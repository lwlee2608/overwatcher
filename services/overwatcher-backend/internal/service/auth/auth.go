package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
	"github.com/lwlee2608/overwatcher/internal/util"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrNoSession          = errors.New("no active session")
	ErrUserNotFound       = errors.New("user not found")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
)

const minPasswordLen = 8

type Session struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
}

type User struct {
	ID    string
	Email string
	Name  string
}

type Service struct {
	q   *sqlc.Queries
	ttl time.Duration
}

func NewService(pool *pgxpool.Pool, ttl time.Duration) *Service {
	return &Service{q: sqlc.New(pool), ttl: ttl}
}

// Login verifies the email/password and creates a new session. The returned
// token is opaque; callers should set it as the session cookie value.
func (s *Service) Login(ctx context.Context, email, password string) (*Session, error) {
	row, err := s.q.GetUserPasswordHashByEmail(ctx, strings.TrimSpace(email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if row.PasswordHash == "" {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(row.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return s.createSession(ctx, row.ID)
}

// Validate looks up the session by token. If the session is within half the
// TTL of expiry, the expiry is slid forward — keeps active users logged in
// without writing on every request.
func (s *Service) Validate(ctx context.Context, token string) (*Session, error) {
	row, err := s.q.GetSession(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoSession
		}
		return nil, err
	}
	sess := &Session{
		Token:     row.Token,
		UserID:    util.UUIDToString(row.UserID),
		ExpiresAt: row.ExpiresAt.Time,
	}
	if time.Until(sess.ExpiresAt) < s.ttl/2 {
		newExpiry := time.Now().Add(s.ttl)
		if err := s.q.RefreshSession(ctx, sqlc.RefreshSessionParams{
			Token:     token,
			ExpiresAt: pgtype.Timestamptz{Time: newExpiry, Valid: true},
		}); err != nil {
			return nil, err
		}
		sess.ExpiresAt = newExpiry
	}
	return sess, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.q.DeleteSession(ctx, token)
}

// ReapExpiredSessions deletes every session row whose expiry is in the
// past. Cheap thanks to idx_sessions_expires_at; intended to be called
// periodically from a background goroutine.
func (s *Service) ReapExpiredSessions(ctx context.Context) error {
	return s.q.DeleteExpiredSessions(ctx)
}

// GetUser returns the public profile for a session's user.
func (s *Service) GetUser(ctx context.Context, userID string) (*User, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, err
	}
	row, err := s.q.GetUser(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &User{
		ID:    util.UUIDToString(row.ID),
		Email: row.Email,
		Name:  row.Name,
	}, nil
}

// ChangePassword verifies the current password and sets a new one.
func (s *Service) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	if len(newPassword) < minPasswordLen {
		return ErrPasswordTooShort
	}
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return err
	}
	row, err := s.q.GetUser(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return err
	}
	if row.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(row.PasswordHash), []byte(oldPassword)) != nil {
		return ErrInvalidCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.q.SetUserPasswordHash(ctx, sqlc.SetUserPasswordHashParams{
		ID:           uid,
		PasswordHash: string(hash),
	}); err != nil {
		return err
	}
	// Revoke every session for this user so old cookies stop working.
	return s.q.DeleteSessionsForUser(ctx, uid)
}

// EnsureUserPassword upserts a user with the given email and sets the
// password. Used by the env-var bootstrap on startup. Idempotent. Existing
// sessions for the user are revoked when the hash changes so a rotated
// bootstrap password kicks attackers (and the operator) out cleanly.
func (s *Service) EnsureUserPassword(ctx context.Context, cfg BootstrapConfig) error {
	if len(cfg.Password) < minPasswordLen {
		return ErrPasswordTooShort
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	row, err := s.q.GetUserByEmail(ctx, cfg.Email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		row, err = s.q.CreateUser(ctx, sqlc.CreateUserParams{Email: cfg.Email, Name: cfg.Name})
		if err != nil {
			return err
		}
	} else if bcrypt.CompareHashAndPassword([]byte(row.PasswordHash), []byte(cfg.Password)) == nil {
		// Hash already matches the desired password; nothing to do.
		return nil
	}
	if err := s.q.SetUserPasswordHash(ctx, sqlc.SetUserPasswordHashParams{
		ID:           row.ID,
		PasswordHash: string(hash),
	}); err != nil {
		return err
	}
	return s.q.DeleteSessionsForUser(ctx, row.ID)
}

func (s *Service) createSession(ctx context.Context, userID pgtype.UUID) (*Session, error) {
	token, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().Add(s.ttl)
	if err := s.q.CreateSession(ctx, sqlc.CreateSessionParams{
		Token:     token,
		UserID:    userID,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		return nil, err
	}
	return &Session{
		Token:     token,
		UserID:    util.UUIDToString(userID),
		ExpiresAt: expiresAt,
	}, nil
}

func newSessionToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
