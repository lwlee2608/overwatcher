-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, password_is_bootstrap)
VALUES ($1, $2, $3, $4)
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

-- name: GetUserPasswordHashByEmail :one
SELECT id, email, password_hash FROM users WHERE LOWER(email) = LOWER($1);

-- name: SetUserPasswordHash :exec
UPDATE users
SET password_hash         = $2,
    password_is_bootstrap = $3,
    updated_at            = NOW()
WHERE id = $1;
