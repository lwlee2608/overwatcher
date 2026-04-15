-- name: CreateEventLog :one
INSERT INTO event_logs (delivery_id, event_type, repo, sender, summary)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListEventLogs :many
SELECT * FROM event_logs
ORDER BY created_at DESC
LIMIT $1;
