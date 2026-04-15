package eventlog

import (
	"context"
	"log/slog"
	"time"

	"github.com/lwlee2608/overwatcher/internal/db/sqlc"
	"github.com/lwlee2608/overwatcher/internal/util"
)

const maxEventLogs = 1000

type Event struct {
	ID         string
	DeliveryID string
	EventType  string
	Repo       string
	Sender     string
	Summary    string
	CreatedAt  time.Time
}

type Service struct {
	queries sqlc.Querier
}

func NewService(queries sqlc.Querier) *Service {
	return &Service{queries: queries}
}

func (s *Service) Record(ctx context.Context, deliveryID, eventType, repo, sender, summary string) error {
	_, err := s.queries.CreateEventLog(ctx, sqlc.CreateEventLogParams{
		DeliveryID: deliveryID,
		EventType:  eventType,
		Repo:       repo,
		Sender:     sender,
		Summary:    summary,
	})
	if err != nil {
		return err
	}

	if n, err := s.queries.DeleteOldEventLogs(ctx, maxEventLogs); err != nil {
		slog.Warn("Failed to trim old event logs", "error", err)
	} else if n > 0 {
		slog.Info("Trimmed old event logs", "deleted", n)
	}

	return nil
}

func (s *Service) List(ctx context.Context, limit int32) ([]Event, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := s.queries.ListEventLogs(ctx, limit)
	if err != nil {
		return nil, err
	}

	events := make([]Event, len(rows))
	for i, r := range rows {
		events[i] = Event{
			ID:         util.UUIDToString(r.ID),
			DeliveryID: r.DeliveryID,
			EventType:  r.EventType,
			Repo:       r.Repo,
			Sender:     r.Sender,
			Summary:    r.Summary,
			CreatedAt:  r.CreatedAt.Time,
		}
	}
	return events, nil
}
