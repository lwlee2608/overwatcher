-- name: UpsertAgent :one
-- agent_type and version are passed in (empty string treated as NULL). On
-- conflict we preserve the existing value when the caller didn't send one,
-- so an old agent reconnecting without the header doesn't wipe known data.
INSERT INTO agents (name, remote_ip, agent_type, version, last_seen_at)
VALUES ($1, $2, NULLIF(sqlc.arg(agent_type)::text, ''), NULLIF(sqlc.arg(version)::text, ''), NOW())
ON CONFLICT (name)
DO UPDATE SET
    remote_ip     = EXCLUDED.remote_ip,
    agent_type    = COALESCE(EXCLUDED.agent_type, agents.agent_type),
    version       = COALESCE(EXCLUDED.version, agents.version),
    last_seen_at  = NOW(),
    updated_at    = NOW()
RETURNING *;

-- name: GetAgent :one
SELECT a.*, p.name AS project_name
FROM agents a
LEFT JOIN projects p ON p.id = a.project_id
WHERE a.id = $1;

-- name: ListAgents :many
SELECT a.*, p.name AS project_name
FROM agents a
LEFT JOIN projects p ON p.id = a.project_id
ORDER BY a.name ASC;

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

-- name: DeleteAgent :execrows
DELETE FROM agents WHERE id = $1;
