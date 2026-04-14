-- name: UpsertAgent :one
INSERT INTO agents (name, compose_file, remote_ip, last_seen_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (name)
DO UPDATE SET
    compose_file  = EXCLUDED.compose_file,
    remote_ip     = EXCLUDED.remote_ip,
    last_seen_at  = NOW(),
    updated_at    = NOW()
RETURNING *;

-- name: GetAgent :one
SELECT * FROM agents WHERE id = $1;

-- name: ListAgents :many
SELECT * FROM agents ORDER BY name ASC;
