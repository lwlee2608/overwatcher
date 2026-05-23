-- name: CreateService :one
INSERT INTO services (project_id, name, repo, root_directory, branch, image, tag, workflow, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetService :one
SELECT * FROM services WHERE id = $1;

-- name: ListServicesByProject :many
SELECT * FROM services
WHERE project_id = $1
ORDER BY position ASC, name ASC;

-- name: DeleteServicesByProject :exec
DELETE FROM services WHERE project_id = $1;

-- name: DeleteServiceInProject :one
DELETE FROM services WHERE id = $1 AND project_id = $2 RETURNING *;

-- name: ListEnabledServicesByRepoAndBranch :many
-- Webhook matching: returns every service in an enabled project whose repo
-- matches (case-insensitive). The webhook handler still has to filter by
-- root_directory against the pushed file paths before enqueueing intents.
SELECT s.*,
       p.name                 AS project_name,
       p.user_id              AS project_user_id,
       p.compose_file         AS project_compose_file,
       p.compose_project_name AS project_compose_project_name,
       p.environment          AS project_environment
FROM services s
JOIN projects p ON p.id = s.project_id
WHERE LOWER(s.repo) = LOWER($1)
  AND s.branch = $2
  AND p.enabled = true
ORDER BY p.id, s.position;

-- name: ListEnabledServicesByRepoAndWorkflow :many
-- workflow_run matching: like ListEnabledServicesByRepoAndBranch but keyed on
-- the workflow filename (e.g. "build-and-publish.yml"). Used when a CI run
-- finishes successfully and we want to deploy the matching services.
SELECT s.*,
       p.name                 AS project_name,
       p.user_id              AS project_user_id,
       p.compose_file         AS project_compose_file,
       p.compose_project_name AS project_compose_project_name,
       p.environment          AS project_environment
FROM services s
JOIN projects p ON p.id = s.project_id
WHERE LOWER(s.repo) = LOWER($1)
  AND s.branch = $2
  AND s.workflow = $3
  AND p.enabled = true
ORDER BY p.id, s.position;
