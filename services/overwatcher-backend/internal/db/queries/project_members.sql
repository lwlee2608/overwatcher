-- name: AddProjectMember :one
INSERT INTO project_members (project_id, user_id, role, added_by)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: RemoveProjectMember :one
DELETE FROM project_members
WHERE project_id = $1 AND user_id = $2
RETURNING *;

-- name: GetProjectMember :one
SELECT * FROM project_members
WHERE project_id = $1 AND user_id = $2;

-- name: ListProjectMembers :many
SELECT pm.project_id, pm.user_id, pm.role, pm.added_by, pm.created_at,
       u.email AS user_email, u.name AS user_name
FROM project_members pm
JOIN users u ON u.id = pm.user_id
WHERE pm.project_id = $1
ORDER BY u.email ASC;

-- name: ListProjectsForUser :many
SELECT p.*, u.email AS user_email,
       CASE WHEN p.user_id = $1 THEN 'owner' ELSE 'member' END AS role
FROM projects p
JOIN users u ON u.id = p.user_id
LEFT JOIN project_members pm
  ON pm.project_id = p.id AND pm.user_id = $1
WHERE p.user_id = $1 OR pm.user_id = $1
ORDER BY p.name ASC;
