-- name: NextVersion :one
SELECT COALESCE(MAX(version), 0) + 1
FROM prompts
WHERE slug = $1
  AND project_id IS NOT DISTINCT FROM $2
  AND deleted_at IS NULL;

-- name: DeactivatePriorVersions :exec
UPDATE prompts SET is_active = false
WHERE slug = $1
  AND project_id IS NOT DISTINCT FROM $2
  AND is_active = true AND deleted_at IS NULL;

-- name: DeactivateOthers :exec
UPDATE prompts SET is_active = false
WHERE slug = $1
  AND project_id IS NOT DISTINCT FROM $2
  AND id <> $3 AND is_active = true AND deleted_at IS NULL;

-- name: InsertPrompt :one
INSERT INTO prompts (project_id, created_by, slug, version,
                     body, variables, description, is_active, tags)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, organization_id, project_id, created_by, slug, version,
          body, variables, COALESCE(description,'')::text AS description, is_active,
          parent_version_id, tags, created_at, updated_at;

-- name: GetByID :one
SELECT id, organization_id, project_id, created_by, slug, version,
       body, variables, COALESCE(description,'')::text AS description, is_active,
       parent_version_id, tags, created_at, updated_at
FROM prompts
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetByIDForUpdate :one
SELECT id, organization_id, project_id, created_by, slug, version,
       body, variables, COALESCE(description,'')::text AS description, is_active,
       parent_version_id, tags, created_at, updated_at
FROM prompts
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE;

-- name: GetActive :one
SELECT id, organization_id, project_id, created_by, slug, version,
       body, variables, COALESCE(description,'')::text AS description, is_active,
       parent_version_id, tags, created_at, updated_at
FROM prompts
WHERE slug = $1
  AND project_id IS NOT DISTINCT FROM $2
  AND is_active = true AND deleted_at IS NULL
ORDER BY version DESC
LIMIT 1;

-- name: ActivatePrompt :exec
UPDATE prompts SET is_active = true
WHERE id = $1;

-- name: ListVersions :many
SELECT id, organization_id, project_id, created_by, slug, version,
       body, variables, COALESCE(description,'')::text AS description, is_active,
       parent_version_id, tags, created_at, updated_at
FROM prompts
WHERE slug = $1
  AND project_id IS NOT DISTINCT FROM $2
  AND deleted_at IS NULL
ORDER BY version DESC;

-- name: SearchPrompts :many
SELECT p.id, p.organization_id, p.project_id, p.created_by, p.slug, p.version,
       p.body, p.variables, COALESCE(p.description,'')::text AS description, p.is_active,
       p.parent_version_id, p.tags, p.created_at, p.updated_at,
       ts_rank(p.body_tsv, q)::float8 AS score,
       ts_headline('spanish', p.body, q, 'StartSel=<mark>,StopSel=</mark>,MaxWords=20,MinWords=5')::text AS headline
FROM prompts p, plainto_tsquery('spanish', sqlc.arg('query')::text) AS q
WHERE p.deleted_at IS NULL AND p.body_tsv @@ q
ORDER BY score DESC
LIMIT sqlc.arg('result_limit')::int;

-- name: SoftDeletePrompt :execrows
UPDATE prompts SET deleted_at = NOW(), is_active = false
WHERE id = $1 AND deleted_at IS NULL;
