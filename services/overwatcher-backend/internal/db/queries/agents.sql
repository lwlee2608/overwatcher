-- name: UpsertAgent :one
INSERT INTO agents (name, remote_ip, last_seen_at)
VALUES ($1, $2, NOW())
ON CONFLICT (name)
DO UPDATE SET
    remote_ip     = EXCLUDED.remote_ip,
    last_seen_at  = NOW(),
    updated_at    = NOW()
RETURNING *;

-- name: GetAgent :one
SELECT * FROM agents WHERE id = $1;

-- name: ListAgents :many
SELECT * FROM agents ORDER BY name ASC;

-- name: GetAgentByName :one
SELECT * FROM agents WHERE name = $1;

-- name: ClearAgentProjectBinding :exec
UPDATE agents
SET project_id = NULL,
    updated_at = NOW()
WHERE project_id = $1;

-- name: BindAgentToProject :one
UPDATE agents
SET project_id = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;
