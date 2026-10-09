-- name: CreateAPIKey :one
INSERT INTO api_keys (user_id, name, token_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListAPIKeysByUser :many
SELECT * FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC;

-- name: GetAPIKeyByTokenHash :one
SELECT * FROM api_keys WHERE token_hash = $1;

-- name: TouchAPIKey :exec
-- Throttled to one write per minute so a busy key doesn't write on every request.
UPDATE api_keys
SET last_used_at = NOW()
WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < NOW() - INTERVAL '1 minute');

-- name: DeleteAPIKeyForUser :one
DELETE FROM api_keys WHERE id = $1 AND user_id = $2 RETURNING *;
