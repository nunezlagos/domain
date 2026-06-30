-- name: InsertPolicy :one
INSERT INTO project_policies
   (project_id, slug, name, kind,
    body_md, body_structured, override_platform, source)
 VALUES (sqlc.arg('project_id'), sqlc.arg('slug'), sqlc.arg('name'),
         sqlc.arg('kind'), sqlc.arg('body_md'), sqlc.arg('body_structured'),
         sqlc.arg('override_platform'), sqlc.arg('source'))
 RETURNING id, project_id, slug, name, kind,
           body_md, body_structured, version, is_active, override_platform,
           source, created_at, updated_at, deleted_at;

-- name: ListPolicies :many
SELECT id, project_id, slug, name, kind,
       body_md, body_structured, version, is_active, override_platform,
       source, created_at, updated_at, deleted_at
FROM project_policies
WHERE project_id = sqlc.arg('project_id')
  AND is_active = TRUE AND deleted_at IS NULL AND proposed = false
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind')::text)
ORDER BY kind ASC, slug ASC;

-- name: GetPolicyBySlug :one
SELECT id, project_id, slug, name, kind,
       body_md, body_structured, version, is_active, override_platform,
       source, created_at, updated_at, deleted_at
FROM project_policies
WHERE project_id = sqlc.arg('project_id') AND slug = sqlc.arg('slug')
  AND is_active = TRUE AND deleted_at IS NULL AND proposed = false;

-- name: GetPolicy :one
SELECT id, project_id, slug, name, kind,
       body_md, body_structured, version, is_active, override_platform,
       source, created_at, updated_at, deleted_at
FROM project_policies
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: UpdatePolicy :one
UPDATE project_policies
SET name = sqlc.arg('name'), kind = sqlc.arg('kind'),
    body_md = sqlc.arg('body_md'), body_structured = sqlc.arg('body_structured'),
    override_platform = sqlc.arg('override_platform'),
    version = sqlc.arg('version')
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, project_id, slug, name, kind,
          body_md, body_structured, version, is_active, override_platform,
          source, created_at, updated_at, deleted_at;

-- name: SoftDeletePolicy :execrows
UPDATE project_policies
SET deleted_at = NOW(), is_active = FALSE
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;
