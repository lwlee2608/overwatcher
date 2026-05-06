-- name: CreateProject :one
INSERT INTO projects (user_id, name, description, compose_file, environment, enabled)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1;

-- name: GetProjectByUserAndName :one
SELECT * FROM projects WHERE user_id = $1 AND name = $2;

-- name: UpdateProject :one
UPDATE projects
SET name         = $2,
    description  = $3,
    compose_file = $4,
    environment  = $5,
    enabled      = $6,
    updated_at   = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteProject :one
DELETE FROM projects WHERE id = $1 RETURNING *;
