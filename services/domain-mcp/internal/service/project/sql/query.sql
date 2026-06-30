-- name: InsertProject :one
INSERT INTO projects (name, slug, description, repository_url, template_id, settings, client_id)
VALUES (
    sqlc.arg('name'),
    sqlc.arg('slug'),
    sqlc.narg('description'),
    sqlc.narg('repository_url'),
    sqlc.narg('template_id'),
    sqlc.arg('settings'),
    sqlc.narg('client_id')
)
RETURNING id;

-- name: GetProjectByID :one
SELECT
    p.id,
    p.name,
    p.slug,
    COALESCE(p.description, '')::text AS description,
    COALESCE(p.repository_url, '')::text AS repository_url,
    p.template_id,
    p.settings,
    p.client_id,
    COALESCE(c.slug, '')::text AS client_slug,
    COALESCE(c.name, '')::text AS client_name,
    p.created_at,
    p.updated_at,
    p.deleted_at
FROM projects p
LEFT JOIN project_clients c ON c.id = p.client_id
WHERE p.id = sqlc.arg('id');

-- name: GetProjectBySlug :one
SELECT
    p.id,
    p.name,
    p.slug,
    COALESCE(p.description, '')::text AS description,
    COALESCE(p.repository_url, '')::text AS repository_url,
    p.template_id,
    p.settings,
    p.client_id,
    COALESCE(c.slug, '')::text AS client_slug,
    COALESCE(c.name, '')::text AS client_name,
    p.created_at,
    p.updated_at,
    p.deleted_at
FROM projects p
LEFT JOIN project_clients c ON c.id = p.client_id
WHERE p.slug = sqlc.arg('slug') AND p.deleted_at IS NULL;

-- name: ListProjects :many
SELECT
    p.id,
    p.name,
    p.slug,
    COALESCE(p.description, '')::text AS description,
    COALESCE(p.repository_url, '')::text AS repository_url,
    p.template_id,
    p.settings,
    p.client_id,
    COALESCE(c.slug, '')::text AS client_slug,
    COALESCE(c.name, '')::text AS client_name,
    p.created_at,
    p.updated_at,
    p.deleted_at
FROM projects p
LEFT JOIN project_clients c ON c.id = p.client_id
WHERE p.deleted_at IS NULL
    AND (sqlc.narg('client_id')::uuid IS NULL OR p.client_id = sqlc.narg('client_id')::uuid)
ORDER BY p.created_at DESC;

-- name: UpdateProject :exec
UPDATE projects
SET name = sqlc.arg('name'),
    description = sqlc.narg('description'),
    repository_url = sqlc.narg('repository_url'),
    settings = sqlc.arg('settings')
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: UpdateProjectWithClient :exec
UPDATE projects
SET name = sqlc.arg('name'),
    description = sqlc.narg('description'),
    repository_url = sqlc.narg('repository_url'),
    settings = sqlc.arg('settings'),
    client_id = sqlc.narg('client_id')
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: SoftDeleteProject :exec
UPDATE projects SET deleted_at = NOW() WHERE id = sqlc.arg('id') AND deleted_at IS NULL;
