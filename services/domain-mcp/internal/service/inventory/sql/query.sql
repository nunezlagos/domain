-- name: ListMCPProviders :many
SELECT name, description, command, tags, required_env
FROM mcp_providers
ORDER BY name;

-- name: ListMCPServers :many
SELECT name, transport, status, enabled
FROM mcp_servers
WHERE enabled = TRUE
ORDER BY name;

-- name: ListProjectTemplates :many
SELECT slug, name, COALESCE(description, '')::text AS description
FROM project_templates
ORDER BY slug;

-- name: ListPlatformPolicies :many
SELECT slug, name, LEFT(COALESCE(body_md, ''), 120)::text AS description
FROM platform_policies
WHERE is_active = TRUE
ORDER BY slug;

-- name: ListAgents :many
SELECT slug, name, COALESCE(description, '')::text AS description,
       COALESCE(model, '')::text AS model,
       COALESCE(skills_slugs, '{}')::text[] AS skills_slugs
FROM agents
WHERE deleted_at IS NULL
ORDER BY slug;

-- name: ListSkills :many
SELECT slug, name, COALESCE(description, '')::text AS description,
       COALESCE(skill_type, 'prompt')::text AS skill_type
FROM skills
WHERE deleted_at IS NULL
ORDER BY slug;

-- name: ListFlows :many
SELECT slug, name, COALESCE(description, '')::text AS description
FROM flows
WHERE deleted_at IS NULL
ORDER BY slug;
