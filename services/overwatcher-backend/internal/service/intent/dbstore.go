package intent

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
)

var _ Store = (*DBStore)(nil)

// DBStore is a PostgreSQL-backed implementation of Store. Intents survive
// coordinator restarts and webhook redeliveries are deduped via a unique
// constraint on (delivery_id, stack_index).
type DBStore struct {
	q      *sqlc.Queries
	notify chan struct{}
}

func NewDBStore(pool *pgxpool.Pool) *DBStore {
	return &DBStore{
		q:      sqlc.New(pool),
		notify: make(chan struct{}, 1),
	}
}

func (s *DBStore) Enqueue(i *DeployIntent) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.q.CreateDeployIntent(ctx, sqlc.CreateDeployIntentParams{
		DeliveryID:     i.DeliveryID,
		StackIndex:     int32(i.StackIndex),
		Repo:           i.Repo,
		GitRef:         i.Ref,
		Sha:            i.SHA,
		Image:          i.Image,
		Tag:            i.Tag,
		Stack:          i.Stack,
		Services:       i.Services,
		Environment:    i.Environment,
		DeploymentID:   i.DeploymentID,
		InstallationID: i.InstallationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Info("Duplicate webhook delivery, skipping", "delivery_id", i.DeliveryID, "stack_index", i.StackIndex)
			return
		}
		slog.Error("Failed to enqueue intent", "delivery_id", i.DeliveryID, "error", err)
		return
	}

	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *DBStore) TakeNext(ctx context.Context) (*DeployIntent, error) {
	for {
		row, err := s.q.TakeNextDeployIntent(ctx)
		if err == nil {
			return fromRow(row), nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}

		select {
		case <-s.notify:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (s *DBStore) Complete(id string, success bool) (*DeployIntent, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status := string(StatusSucceeded)
	if !success {
		status = string(StatusFailed)
	}

	row, err := s.q.CompleteDeployIntent(ctx, sqlc.CompleteDeployIntentParams{
		ID:     parseUUID(id),
		Status: status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false
		}
		slog.Error("Failed to complete intent", "intent_id", id, "error", err)
		return nil, false
	}

	// Wake blocked TakeNext callers — a stack slot freed up.
	select {
	case s.notify <- struct{}{}:
	default:
	}
	return fromRow(row), true
}

func (s *DBStore) Requeue(id string) (*DeployIntent, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row, err := s.q.RequeueDeployIntent(ctx, parseUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false
		}
		slog.Error("Failed to requeue intent", "intent_id", id, "error", err)
		return nil, false
	}

	select {
	case s.notify <- struct{}{}:
	default:
	}
	return fromRow(row), true
}

func (s *DBStore) SweepTimedOut(timeout time.Duration, maxAttempts int) (requeued, failed []*DeployIntent) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cutoff := pgtype.Timestamp{Time: time.Now().Add(-timeout), Valid: true}

	requeuedRows, err := s.q.RequeueTimedOutIntents(ctx, sqlc.RequeueTimedOutIntentsParams{
		Cutoff:      cutoff,
		MaxAttempts: int32(maxAttempts),
	})
	if err != nil {
		slog.Error("Failed to requeue timed out intents", "error", err)
	}
	for _, row := range requeuedRows {
		requeued = append(requeued, fromRow(row))
	}

	failedRows, err := s.q.FailTimedOutIntents(ctx, sqlc.FailTimedOutIntentsParams{
		Cutoff:      cutoff,
		MaxAttempts: int32(maxAttempts),
	})
	if err != nil {
		slog.Error("Failed to mark timed out intents as failed", "error", err)
	}
	for _, row := range failedRows {
		failed = append(failed, fromRow(row))
	}

	if len(requeued) > 0 {
		select {
		case s.notify <- struct{}{}:
		default:
		}
	}
	return requeued, failed
}

func (s *DBStore) List() []*DeployIntent {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := s.q.ListDeployIntentsByStatus(ctx, string(StatusCreated))
	if err != nil {
		slog.Error("Failed to list intents", "error", err)
		return nil
	}
	out := make([]*DeployIntent, len(rows))
	for i, row := range rows {
		out[i] = fromRow(row)
	}
	return out
}

func (s *DBStore) InFlight() []*DeployIntent {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := s.q.ListDeployIntentsByStatus(ctx, string(StatusDispatched))
	if err != nil {
		slog.Error("Failed to list in-flight intents", "error", err)
		return nil
	}
	out := make([]*DeployIntent, len(rows))
	for i, row := range rows {
		out[i] = fromRow(row)
	}
	return out
}

func (s *DBStore) Len() int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	count, err := s.q.CountDeployIntentsByStatus(ctx, string(StatusCreated))
	if err != nil {
		slog.Error("Failed to count intents", "error", err)
		return 0
	}
	return int(count)
}

// fromRow converts a sqlc-generated DeployIntent to the domain model.
func fromRow(row sqlc.DeployIntent) *DeployIntent {
	di := &DeployIntent{
		ID:             uuidToString(row.ID),
		DeliveryID:     row.DeliveryID,
		StackIndex:     int(row.StackIndex),
		Repo:           row.Repo,
		Ref:            row.GitRef,
		SHA:            row.Sha,
		Image:          row.Image,
		Tag:            row.Tag,
		Stack:          row.Stack,
		Services:       row.Services,
		Environment:    row.Environment,
		DeploymentID:   row.DeploymentID,
		InstallationID: row.InstallationID,
		Status:         Status(row.Status),
		Attempts:       int(row.Attempts),
	}
	if row.CreatedAt.Valid {
		di.CreatedAt = row.CreatedAt.Time
	}
	if row.DispatchedAt.Valid {
		di.DispatchedAt = row.DispatchedAt.Time
	}
	return di
}

func parseUUID(s string) pgtype.UUID {
	u, err := uuid.Parse(s)
	if err != nil {
		slog.Error("Invalid UUID", "id", s, "error", err)
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: u, Valid: true}
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}
