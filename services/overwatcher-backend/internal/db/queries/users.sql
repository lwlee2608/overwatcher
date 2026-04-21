-- name: CreateUser :one
INSERT INTO users (email, name)
VALUES ($1, $2)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE LOWER(email) = LOWER($1);

-- name: ListUsers :many
SELECT * FROM users ORDER BY email ASC;

-- name: UpdateUser :one
UPDATE users
SET email      = $2,
    name       = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteUser :one
DELETE FROM users WHERE id = $1 RETURNING *;
