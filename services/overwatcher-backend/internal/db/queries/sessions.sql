-- name: CreateSession :exec
INSERT INTO sessions (token, user_id, expires_at)
VALUES ($1, $2, $3);

-- name: GetSession :one
SELECT token, user_id, expires_at, created_at
FROM sessions
WHERE token = $1 AND expires_at > NOW();

-- name: RefreshSession :exec
UPDATE sessions
SET expires_at = $2
WHERE token = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = $1;

-- name: DeleteSessionsForUser :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= NOW();
