-- name: InsertPolicy :one
INSERT INTO platform_policies (slug, name, kind, body_md, body_structured, source_file)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, slug, name, kind, body_md, body_structured, version, is_active, source_file, created_at, updated_at, is_user_modified;

-- name: GetActivePolicyBySlug :one
SELECT id, slug, name, kind, body_md, body_structured, version, is_active, source_file, created_at, updated_at, is_user_modified
FROM platform_policies WHERE slug = $1 AND is_active = TRUE;

-- name: GetPolicyForUpdate :one
SELECT id, slug, name, kind, body_md, body_structured, version, is_active, source_file, created_at, updated_at, is_user_modified
FROM platform_policies WHERE id = $1 FOR UPDATE;

-- name: ListActivePolicies :many
SELECT id, slug, name, kind, body_md, body_structured, version, is_active, source_file, created_at, updated_at, is_user_modified
FROM platform_policies
WHERE is_active = TRUE
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind')::text)
ORDER BY kind, slug;

-- name: InsertPolicyVersion :exec
INSERT INTO platform_policy_versions (policy_id, version, body_md, body_structured, changed_by)
VALUES ($1, $2, $3, $4, $5);

-- name: UpdatePolicyBody :exec
UPDATE platform_policies
SET body_md = $2, body_structured = $3, version = version + 1, is_user_modified = TRUE
WHERE id = $1;

-- name: DeactivatePolicy :execrows
UPDATE platform_policies SET is_active = FALSE WHERE id = $1 AND is_active = TRUE;
