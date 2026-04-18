-- name: ListDeployMappings :many
SELECT dm.*,
       a.name AS agent_name,
       COALESCE(
           (SELECT jsonb_agg(
                       jsonb_build_object('name', s.name, 'image', s.image, 'tag', s.tag)
                       ORDER BY s.position
                   )
            FROM deploy_mapping_services s
            WHERE s.mapping_id = dm.id),
           '[]'::jsonb
       ) AS services
FROM deploy_mappings dm
JOIN agents a ON a.id = dm.agent_id
ORDER BY dm.repo ASC, dm.created_at ASC;

-- name: GetDeployMapping :one
SELECT dm.*,
       a.name AS agent_name,
       COALESCE(
           (SELECT jsonb_agg(
                       jsonb_build_object('name', s.name, 'image', s.image, 'tag', s.tag)
                       ORDER BY s.position
                   )
            FROM deploy_mapping_services s
            WHERE s.mapping_id = dm.id),
           '[]'::jsonb
       ) AS services
FROM deploy_mappings dm
JOIN agents a ON a.id = dm.agent_id
WHERE dm.id = $1;

-- name: CreateDeployMapping :one
INSERT INTO deploy_mappings (repo, agent_id, environment, enabled)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateDeployMapping :one
UPDATE deploy_mappings
SET repo        = $2,
    agent_id    = $3,
    environment = $4,
    enabled     = $5,
    updated_at  = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteDeployMapping :one
DELETE FROM deploy_mappings WHERE id = $1 RETURNING *;

-- name: ListEnabledMappingsByRepo :many
SELECT dm.*,
       a.name AS agent_name,
       COALESCE(
           (SELECT jsonb_agg(
                       jsonb_build_object('name', s.name, 'image', s.image, 'tag', s.tag)
                       ORDER BY s.position
                   )
            FROM deploy_mapping_services s
            WHERE s.mapping_id = dm.id),
           '[]'::jsonb
       ) AS services
FROM deploy_mappings dm
JOIN agents a ON a.id = dm.agent_id
WHERE LOWER(dm.repo) = LOWER($1)
  AND dm.enabled = true
ORDER BY dm.created_at ASC;

-- name: CreateMappingService :exec
INSERT INTO deploy_mapping_services (mapping_id, name, image, tag, position)
VALUES ($1, $2, $3, $4, $5);

-- name: DeleteMappingServicesByMapping :exec
DELETE FROM deploy_mapping_services WHERE mapping_id = $1;
