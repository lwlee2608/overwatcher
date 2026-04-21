-- name: CreateService :one
INSERT INTO services (project_id, name, repo, root_directory, branch, image, tag, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetService :one
SELECT * FROM services WHERE id = $1;

-- name: ListServicesByProject :many
SELECT * FROM services
WHERE project_id = $1
ORDER BY position ASC, name ASC;

-- name: DeleteServicesByProject :exec
DELETE FROM services WHERE project_id = $1;

-- name: DeleteService :one
DELETE FROM services WHERE id = $1 RETURNING *;

-- name: ListEnabledServicesByRepoAndBranch :many
-- Webhook matching: returns every service in an enabled project whose repo
-- matches (case-insensitive). The webhook handler still has to filter by
-- root_directory against the pushed file paths before enqueueing intents.
SELECT s.*,
       p.name         AS project_name,
       p.user_id      AS project_user_id,
       p.compose_file AS project_compose_file,
       p.environment  AS project_environment
FROM services s
JOIN projects p ON p.id = s.project_id
WHERE LOWER(s.repo) = LOWER($1)
  AND s.branch = $2
  AND p.enabled = true
ORDER BY p.id, s.position;
