-- name: InsertRequirement :one
INSERT INTO sdd_requirements (slug, title, description, status, priority, parent_id, project_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, slug, title, description, status, priority, parent_id, created_at, updated_at, project_id;

-- name: GetRequirementBySlug :one
SELECT id, slug, title, description, status, priority, parent_id, created_at, updated_at, project_id
FROM sdd_requirements WHERE slug = $1;

-- name: GetRequirementByID :one
SELECT id, slug, title, description, status, priority, parent_id, created_at, updated_at, project_id
FROM sdd_requirements WHERE id = $1;

-- name: ListRequirements :many
SELECT id, slug, title, description, status, priority, parent_id, created_at, updated_at, project_id
FROM sdd_requirements
WHERE (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('priority')::text IS NULL OR priority = sqlc.narg('priority')::text)
  AND (sqlc.narg('parent_id')::uuid IS NULL OR parent_id = sqlc.narg('parent_id')::uuid)
ORDER BY slug
LIMIT $1 OFFSET $2;

-- name: UpdateRequirement :one
UPDATE sdd_requirements SET title = $2, description = $3, status = $4, priority = $5, updated_at = NOW()
WHERE slug = $1
RETURNING id, slug, title, description, status, priority, parent_id, created_at, updated_at, project_id;

-- name: ArchiveRequirement :exec
UPDATE sdd_requirements SET status = 'archived', updated_at = NOW()
WHERE id = $1;

-- name: ArchiveRequirementRecursive :exec
UPDATE sdd_requirements SET status = 'archived', updated_at = NOW()
WHERE id = $1 OR parent_id = $1;

-- name: GetRequirementTree :many
WITH RECURSIVE req_tree AS (
    SELECT sr.id, sr.slug, sr.title, sr.description, sr.status, sr.priority, sr.parent_id, sr.created_at, sr.updated_at, sr.project_id, 0 AS depth
    FROM sdd_requirements sr WHERE sr.slug = $1
    UNION ALL
    SELECT r.id, r.slug, r.title, r.description, r.status, r.priority, r.parent_id, r.created_at, r.updated_at, r.project_id, rt.depth + 1
    FROM sdd_requirements r
    INNER JOIN req_tree rt ON r.parent_id = rt.id
    WHERE rt.depth < 10
)
SELECT rt.id, rt.slug, rt.title, rt.description, rt.status, rt.priority, rt.parent_id, rt.created_at, rt.updated_at, rt.project_id, rt.depth
FROM req_tree rt ORDER BY rt.depth, rt.slug;
