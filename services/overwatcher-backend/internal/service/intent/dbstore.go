package intent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
)

// DBStore is the PostgreSQL-backed intent queue. Intents survive
// coordinator restarts and webhook redeliveries are deduped via a partial
// unique index on (delivery_id, project_id).
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

	spec, err := json.Marshal(i.Services)
	if err != nil {
		slog.Error("Failed to marshal services spec", "delivery_id", i.DeliveryID, "error", err)
		return
	}

	projectID := pgtype.UUID{}
	if i.ProjectID != "" {
		if err := projectID.Scan(i.ProjectID); err != nil {
			slog.Error("Invalid project_id", "project_id", i.ProjectID, "error", err)
			return
		}
	}

	_, err = s.q.CreateDeployIntent(ctx, sqlc.CreateDeployIntentParams{
		DeliveryID:     i.DeliveryID,
		ProjectID:      projectID,
		Repo:           i.Repo,
		GitRef:         i.Ref,
		Sha:            i.SHA,
		Stack:          i.Stack,
		ServicesSpec:   spec,
		Environment:    i.Environment,
		ComposeFile:    i.ComposeFile,
		DeploymentID:   i.DeploymentID,
		InstallationID: i.InstallationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Info("Duplicate webhook delivery, skipping", "delivery_id", i.DeliveryID, "project_id", i.ProjectID)
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

// TakeNext blocks until an intent bound to agentName's project is available.
// Filtering by agent name (instead of returning the global oldest) keeps one
// agent from claiming another agent's project's intent.
func (s *DBStore) TakeNext(ctx context.Context, agentName string) (*DeployIntent, error) {
	for {
		row, err := s.q.TakeNextDeployIntent(ctx, agentName)
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

	// .UTC() is load-bearing: pgtype.Timestamp's discardTimeZone keeps the
	// wall-clock digits and relabels them as UTC, which shifts the value by
	// the local offset on non-UTC hosts.
	cutoff := pgtype.Timestamp{Time: time.Now().Add(-timeout).UTC(), Valid: true}

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

func (s *DBStore) GetByID(ctx context.Context, id string) (*DeployIntent, error) {
	row, err := s.q.GetDeployIntentByID(ctx, parseUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return fromRow(row), nil
}

func (s *DBStore) ListRecent(ctx context.Context, limit int32) ([]*DeployIntent, error) {
	rows, err := s.q.ListRecentDeployIntents(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*DeployIntent, len(rows))
	for i, row := range rows {
		out[i] = fromRow(row)
	}
	return out, nil
}

func (s *DBStore) ListRecentForUser(ctx context.Context, userID string, limit int32) ([]*DeployIntent, error) {
	uid := pgtype.UUID{}
	if err := uid.Scan(userID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListRecentDeployIntentsForUser(ctx, sqlc.ListRecentDeployIntentsForUserParams{
		UserID: uid,
		Limit:  limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*DeployIntent, len(rows))
	for i, row := range rows {
		out[i] = fromRow(row)
	}
	return out, nil
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
	var services []ServiceSpec
	if len(row.ServicesSpec) > 0 {
		if err := json.Unmarshal(row.ServicesSpec, &services); err != nil {
			slog.Error("Failed to unmarshal services spec", "intent_id", uuidToString(row.ID), "error", err)
		}
	}
	di := &DeployIntent{
		ID:             uuidToString(row.ID),
		DeliveryID:     row.DeliveryID,
		ProjectID:      uuidToString(row.ProjectID),
		ComposeFile:    row.ComposeFile,
		Repo:           row.Repo,
		Ref:            row.GitRef,
		SHA:            row.Sha,
		Stack:          row.Stack,
		Services:       services,
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
