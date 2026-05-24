-- name: UpsertAgent :one
-- agent_type is passed in (empty string treated as NULL). On conflict we
-- preserve the existing type when the caller didn't send one, so an old
-- agent reconnecting without the header doesn't wipe a known type.
INSERT INTO agents (name, remote_ip, agent_type, last_seen_at)
VALUES ($1, $2, NULLIF(sqlc.arg(agent_type)::text, ''), NOW())
ON CONFLICT (name)
DO UPDATE SET
    remote_ip     = EXCLUDED.remote_ip,
    agent_type    = COALESCE(EXCLUDED.agent_type, agents.agent_type),
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
