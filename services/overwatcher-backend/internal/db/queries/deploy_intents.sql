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

-- name: ListDeployIntentsByStatus :many
SELECT * FROM deploy_intents
WHERE status = $1
ORDER BY created_at ASC;

-- name: TakeNextDeployIntent :one
-- Atomically claim the oldest dispatchable intent. The CTE skips stacks that
-- already have a dispatched intent (concurrency guard) and uses FOR UPDATE
-- SKIP LOCKED so concurrent callers don't block each other.
WITH candidate AS (
    SELECT id
    FROM deploy_intents
    WHERE status = 'created'
      AND stack NOT IN (
          SELECT DISTINCT stack FROM deploy_intents WHERE status = 'dispatched'
      )
    ORDER BY created_at ASC
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
