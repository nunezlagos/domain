-- name: InsertTemplate :one
INSERT INTO project_templates
    (slug, name, description, is_default, is_public,
     settings, default_skills, default_agents, default_flows)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, created_at, updated_at;

-- name: GetTemplateByID :one
SELECT id, slug, name, COALESCE(description, '')::text AS description,
    is_default, is_public, settings, default_skills, default_agents,
    default_flows, created_at, updated_at
FROM project_templates
WHERE id = $1;

-- name: GetTemplateBySlug :one
SELECT id, slug, name, COALESCE(description, '')::text AS description,
    is_default, is_public, settings, default_skills, default_agents,
    default_flows, created_at, updated_at
FROM project_templates
WHERE slug = $1;

-- name: ListTemplates :many
SELECT id, slug, name, COALESCE(description, '')::text AS description,
    is_default, is_public, settings, default_skills, default_agents,
    default_flows, created_at, updated_at
FROM project_templates
ORDER BY is_default DESC, name;

-- name: DeleteTemplate :execrows
DELETE FROM project_templates WHERE id = $1;
