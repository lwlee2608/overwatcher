-- name: ListDeployMappings :many
SELECT dm.*,
       a.name AS agent_name
FROM deploy_mappings dm
JOIN agents a ON a.id = dm.agent_id
ORDER BY dm.repo ASC, dm.created_at ASC;

-- name: GetDeployMapping :one
SELECT dm.*,
       a.name AS agent_name
FROM deploy_mappings dm
JOIN agents a ON a.id = dm.agent_id
WHERE dm.id = $1;

-- name: CreateDeployMapping :one
INSERT INTO deploy_mappings (repo, agent_id, services, environment, enabled, image, tag)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateDeployMapping :one
UPDATE deploy_mappings
SET repo        = $2,
    agent_id    = $3,
    services    = $4,
    environment = $5,
    enabled     = $6,
    image       = $7,
    tag         = $8,
    updated_at  = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteDeployMapping :one
DELETE FROM deploy_mappings WHERE id = $1 RETURNING *;

-- name: ListEnabledMappingsByRepo :many
SELECT dm.*,
       a.name AS agent_name
FROM deploy_mappings dm
JOIN agents a ON a.id = dm.agent_id
WHERE LOWER(dm.repo) = LOWER($1)
  AND dm.enabled = true
ORDER BY dm.created_at ASC;
