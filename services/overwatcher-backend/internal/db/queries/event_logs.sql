-- name: CreateEventLog :one
INSERT INTO event_logs (delivery_id, event_type, repo, sender, summary)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListEventLogs :many
SELECT * FROM event_logs
ORDER BY created_at DESC
LIMIT $1;

-- name: ListEventLogsPaged :many
-- Paged + filtered listing. Each filter is nullable; NULL means "no filter".
SELECT * FROM event_logs
WHERE (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type'))
  AND (sqlc.narg('repo')::text IS NULL OR repo ILIKE '%' || sqlc.narg('repo') || '%')
  AND (sqlc.narg('sender')::text IS NULL OR sender = sqlc.narg('sender'))
ORDER BY created_at DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountEventLogs :one
SELECT count(*) FROM event_logs
WHERE (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type'))
  AND (sqlc.narg('repo')::text IS NULL OR repo ILIKE '%' || sqlc.narg('repo') || '%')
  AND (sqlc.narg('sender')::text IS NULL OR sender = sqlc.narg('sender'));

-- name: DeleteOldEventLogs :execrows
DELETE FROM event_logs
WHERE id IN (
    SELECT id FROM event_logs
    ORDER BY created_at DESC
    OFFSET $1
);
