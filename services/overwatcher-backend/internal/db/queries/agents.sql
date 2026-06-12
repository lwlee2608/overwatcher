-- name: CreateAgent :one
-- Pre-provision an agent. token_hash is sha256(raw token); the raw token is
-- returned to the caller once and never stored. Fails on duplicate name.
INSERT INTO agents (name, installed_by_user_id, token_hash, remote_ip)
VALUES ($1, $2, $3, '')
RETURNING *;

-- name: SetAgentToken :one
-- Re-issue: replace the stored digest with a fresh one (migration / loss recovery).
UPDATE agents
SET token_hash = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: TouchAgent :exec
-- Heartbeat on the agent already resolved from its token. Empty agent_type or
-- version preserves the existing value so a poll without the header can't wipe
-- it; NULL metrics likewise keep the last reported values.
UPDATE agents
SET remote_ip        = $2,
    agent_type       = COALESCE(NULLIF(sqlc.arg(agent_type)::text, ''), agent_type),
    version          = COALESCE(NULLIF(sqlc.arg(version)::text, ''), version),
    cpu_percent      = COALESCE(sqlc.narg(cpu_percent)::real, cpu_percent),
    mem_used_bytes   = COALESCE(sqlc.narg(mem_used_bytes)::bigint, mem_used_bytes),
    mem_total_bytes  = COALESCE(sqlc.narg(mem_total_bytes)::bigint, mem_total_bytes),
    disk_used_bytes  = COALESCE(sqlc.narg(disk_used_bytes)::bigint, disk_used_bytes),
    disk_total_bytes = COALESCE(sqlc.narg(disk_total_bytes)::bigint, disk_total_bytes),
    last_seen_at = NOW(),
    updated_at   = NOW()
WHERE id = $1;

-- name: GetAgentByTokenHash :one
SELECT * FROM agents WHERE token_hash = $1;

-- name: GetAgent :one
SELECT a.*, p.name AS project_name
FROM agents a
LEFT JOIN projects p ON p.id = a.project_id
WHERE a.id = $1;

-- name: ListAgentsForUser :many
-- Visibility: an unbound agent is seen only by its installer; once bound, by
-- members of its project (owner or project_members row).
SELECT a.*, p.name AS project_name
FROM agents a
LEFT JOIN projects p ON p.id = a.project_id
WHERE (a.project_id IS NULL AND a.installed_by_user_id = $1)
   OR a.project_id IN (
        SELECT pr.id
        FROM projects pr
        LEFT JOIN project_members pm ON pm.project_id = pr.id AND pm.user_id = $1
        WHERE pr.user_id = $1 OR pm.user_id = $1
   )
ORDER BY a.name ASC;

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
