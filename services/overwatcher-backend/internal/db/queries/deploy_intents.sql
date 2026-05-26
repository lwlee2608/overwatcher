-- name: CreateDeployIntent :one
-- Webhook redeliveries dedup on (delivery_id, project_id) via the partial
-- unique index idx_deploy_intents_delivery_project. On conflict sqlc returns
-- pgx.ErrNoRows, which callers treat as "already enqueued, do nothing".
INSERT INTO deploy_intents (
    delivery_id, project_id, repo, git_ref, sha,
    stack, services_spec, environment, compose_file,
    deployment_id, installation_id
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9,
    $10, $11
)
ON CONFLICT (delivery_id, project_id) WHERE project_id IS NOT NULL DO NOTHING
RETURNING *;

-- name: ListDeployIntentsByStatus :many
SELECT * FROM deploy_intents
WHERE status = $1
ORDER BY created_at ASC;

-- name: GetDeployIntentByID :one
SELECT * FROM deploy_intents WHERE id = $1;

-- name: TakeNextDeployIntent :one
-- Claim the oldest dispatchable intent for @agent_name's bound project.
-- FOR UPDATE SKIP LOCKED + the dispatched-stack guard keep concurrent
-- pollers from blocking each other or double-claiming a stack.
WITH candidate AS (
    SELECT di.id
    FROM deploy_intents di
    JOIN agents a ON a.project_id = di.project_id
    WHERE di.status = 'created'
      AND a.name = @agent_name
      AND di.stack NOT IN (
          SELECT DISTINCT stack FROM deploy_intents WHERE status = 'dispatched'
      )
    ORDER BY di.created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
UPDATE deploy_intents di
SET status = 'dispatched',
    attempts = attempts + 1,
    dispatched_at = NOW(),
    updated_at = NOW()
FROM candidate c
WHERE di.id = c.id
RETURNING di.*;

-- name: CompleteDeployIntent :one
UPDATE deploy_intents
SET status = @status,
    updated_at = NOW()
WHERE id = @id AND status = 'dispatched'
RETURNING *;

-- name: RequeueDeployIntent :one
UPDATE deploy_intents
SET status = 'created',
    dispatched_at = NULL,
    updated_at = NOW()
WHERE id = @id AND status = 'dispatched'
RETURNING *;

-- name: RequeueTimedOutIntents :many
UPDATE deploy_intents
SET status = 'created',
    dispatched_at = NULL,
    updated_at = NOW()
WHERE status = 'dispatched'
  AND dispatched_at < @cutoff
  AND attempts < @max_attempts
RETURNING *;

-- name: FailTimedOutIntents :many
UPDATE deploy_intents
SET status = 'permanently_failed',
    updated_at = NOW()
WHERE status = 'dispatched'
  AND dispatched_at < @cutoff
  AND attempts >= @max_attempts
RETURNING *;

-- name: CountDeployIntentsByStatus :one
SELECT count(*) FROM deploy_intents WHERE status = @status;

-- name: ListRecentDeployIntents :many
SELECT * FROM deploy_intents
ORDER BY created_at DESC
LIMIT $1;

-- name: ListRecentDeployIntentsForUser :many
-- Same as ListRecentDeployIntents but scoped to projects the user can access:
-- either owns (projects.user_id) or is a member of (project_members).
SELECT di.*
FROM deploy_intents di
JOIN projects p ON p.id = di.project_id
LEFT JOIN project_members pm
  ON pm.project_id = p.id AND pm.user_id = $1
WHERE p.user_id = $1 OR pm.user_id = $1
ORDER BY di.created_at DESC
LIMIT $2;

-- name: ListDeployIntentsForUserPaged :many
-- Paged + filtered variant of ListRecentDeployIntentsForUser. Each filter
-- param is nullable; NULL means "no filter on this column".
SELECT di.*
FROM deploy_intents di
JOIN projects p ON p.id = di.project_id
LEFT JOIN project_members pm
  ON pm.project_id = p.id AND pm.user_id = @user_id
WHERE (p.user_id = @user_id OR pm.user_id = @user_id)
  AND (sqlc.narg('status')::text IS NULL OR di.status = sqlc.narg('status'))
  AND (sqlc.narg('project_id')::uuid IS NULL OR di.project_id = sqlc.narg('project_id'))
  AND (sqlc.narg('repo')::text IS NULL OR di.repo ILIKE '%' || sqlc.narg('repo') || '%')
  AND (sqlc.narg('environment')::text IS NULL OR di.environment = sqlc.narg('environment'))
ORDER BY di.created_at DESC, di.id DESC
LIMIT @row_limit OFFSET @row_offset;

-- name: CountDeployIntentsForUser :one
SELECT count(*)
FROM deploy_intents di
JOIN projects p ON p.id = di.project_id
LEFT JOIN project_members pm
  ON pm.project_id = p.id AND pm.user_id = @user_id
WHERE (p.user_id = @user_id OR pm.user_id = @user_id)
  AND (sqlc.narg('status')::text IS NULL OR di.status = sqlc.narg('status'))
  AND (sqlc.narg('project_id')::uuid IS NULL OR di.project_id = sqlc.narg('project_id'))
  AND (sqlc.narg('repo')::text IS NULL OR di.repo ILIKE '%' || sqlc.narg('repo') || '%')
  AND (sqlc.narg('environment')::text IS NULL OR di.environment = sqlc.narg('environment'));
