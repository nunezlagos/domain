-- name: LinkByGitRemote :one
SELECT id::text, organization_id::text, slug
FROM projects
WHERE deleted_at IS NULL
  AND repository_url = ANY(sqlc.arg('candidates')::text[])
ORDER BY array_position(sqlc.arg('candidates')::text[], repository_url)
LIMIT 1;

-- name: UpdateBranch :exec
UPDATE projects
SET current_branch = sqlc.arg('branch'), updated_at = NOW()
WHERE id = sqlc.arg('project_id')::uuid AND deleted_at IS NULL;

-- name: GetRules :one
SELECT rules FROM projects
WHERE id = sqlc.arg('project_id')::uuid AND deleted_at IS NULL;

-- name: SetRules :exec
UPDATE projects
SET rules = sqlc.arg('rules'), updated_at = NOW()
WHERE id = sqlc.arg('project_id')::uuid AND deleted_at IS NULL;
