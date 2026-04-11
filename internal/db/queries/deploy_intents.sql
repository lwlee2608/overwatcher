-- name: CreateDeployIntent :one
-- Webhook redeliveries dedup on (delivery_id, stack_index): on conflict the
-- insert is skipped and sqlc returns pgx.ErrNoRows, which callers treat as
-- "already enqueued, do nothing".
INSERT INTO deploy_intents (
    delivery_id, stack_index, repo, git_ref, sha,
    image, tag, stack, services, environment,
    deployment_id, installation_id
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12
)
ON CONFLICT (delivery_id, stack_index) DO NOTHING
RETURNING *;

-- name: GetDeployIntent :one
SELECT * FROM deploy_intents
WHERE id = $1 LIMIT 1;

-- name: ListDeployIntentsByStatus :many
SELECT * FROM deploy_intents
WHERE status = $1
ORDER BY created_at ASC;

-- name: UpdateDeployIntentStatus :one
UPDATE deploy_intents
SET status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: IncrementDeployIntentAttempts :one
UPDATE deploy_intents
SET attempts = attempts + 1, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteDeployIntent :exec
DELETE FROM deploy_intents
WHERE id = $1;
