-- agent: pg_repository.go

-- name: InsertAgent :one
INSERT INTO agents
  (slug, name, description, provider, model, system_prompt,
   skills_slugs, max_iterations, token_budget, temperature)
VALUES (
  sqlc.arg('slug'), sqlc.arg('name'), sqlc.arg('description'),
  sqlc.arg('provider'), sqlc.arg('model'), sqlc.arg('system_prompt'),
  sqlc.arg('skills_slugs'), sqlc.arg('max_iterations'),
  sqlc.arg('token_budget'), sqlc.arg('temperature')
)
RETURNING id, slug, name, COALESCE(description, '')::text AS description,
          provider, model, COALESCE(system_prompt, '')::text AS system_prompt,
          skills_slugs, max_iterations, token_budget, temperature,
          seed_managed, seed_version, is_user_modified, created_at, updated_at;

-- name: UpdateAgent :one
UPDATE agents
SET name            = sqlc.arg('name'),
    description     = sqlc.arg('description'),
    provider        = sqlc.arg('provider'),
    model           = sqlc.arg('model'),
    system_prompt   = sqlc.arg('system_prompt'),
    skills_slugs    = sqlc.arg('skills_slugs'),
    max_iterations  = sqlc.arg('max_iterations'),
    token_budget    = sqlc.arg('token_budget'),
    temperature     = sqlc.arg('temperature'),
    is_user_modified = sqlc.arg('is_user_modified')
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING id, slug, name, COALESCE(description, '')::text AS description,
          provider, model, COALESCE(system_prompt, '')::text AS system_prompt,
          skills_slugs, max_iterations, token_budget, temperature,
          seed_managed, seed_version, is_user_modified, created_at, updated_at;

-- name: GetAgentByID :one
SELECT id, slug, name, COALESCE(description, '')::text AS description,
       provider, model, COALESCE(system_prompt, '')::text AS system_prompt,
       skills_slugs, max_iterations, token_budget, temperature,
       seed_managed, seed_version, is_user_modified, created_at, updated_at
FROM agents
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: GetAgentBySlug :one
SELECT id, slug, name, COALESCE(description, '')::text AS description,
       provider, model, COALESCE(system_prompt, '')::text AS system_prompt,
       skills_slugs, max_iterations, token_budget, temperature,
       seed_managed, seed_version, is_user_modified, created_at, updated_at
FROM agents
WHERE slug = sqlc.arg('slug') AND deleted_at IS NULL;

-- name: ListAgents :many
SELECT id, slug, name, COALESCE(description, '')::text AS description,
       provider, model, COALESCE(system_prompt, '')::text AS system_prompt,
       skills_slugs, max_iterations, token_budget, temperature,
       seed_managed, seed_version, is_user_modified, created_at, updated_at
FROM agents
WHERE deleted_at IS NULL
ORDER BY created_at DESC
LIMIT sqlc.arg('result_limit')::int;

-- name: SoftDeleteAgent :exec
UPDATE agents SET deleted_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: CountValidSkills :one
SELECT COUNT(*) FROM skills
WHERE slug = ANY(sqlc.arg('slugs')::text[]) AND deleted_at IS NULL;

-- name: AgentSlugTaken :one
SELECT EXISTS (
  SELECT 1 FROM agents
  WHERE slug = sqlc.arg('slug') AND deleted_at IS NULL
);

-- agent_versions

-- name: InsertAgentVersion :exec
INSERT INTO agent_versions (agent_id, version, snapshot, changed_by)
SELECT sqlc.arg('agent_id')::uuid,
       COALESCE((SELECT MAX(av.version) FROM agent_versions av WHERE av.agent_id = sqlc.arg('agent_id')::uuid), 0) + 1,
       sqlc.arg('snapshot'),
       sqlc.arg('changed_by');

-- name: PurgeOldAgentVersions :exec
DELETE FROM agent_versions av
WHERE av.agent_id = sqlc.arg('agent_id')::uuid
  AND av.version <= (
    SELECT MAX(av2.version) - sqlc.arg('max_versions_kept')::int
    FROM agent_versions av2
    WHERE av2.agent_id = sqlc.arg('agent_id')::uuid
  );

-- name: ListAgentVersions :many
SELECT version, snapshot, changed_by, created_at
FROM agent_versions
WHERE agent_id = sqlc.arg('agent_id')
ORDER BY version DESC
LIMIT sqlc.arg('result_limit')::int;

-- agent: orchestration/templates_store.go

-- name: UpsertAgentTemplate :one
INSERT INTO agent_templates
  (slug, name, system_prompt, personality, capabilities,
   model, temperature, max_tokens, handoff_policy, metadata)
VALUES (
  sqlc.arg('slug'), sqlc.arg('name'), sqlc.arg('system_prompt'),
  NULLIF(sqlc.arg('personality'), ''), sqlc.arg('capabilities'),
  sqlc.arg('model'), sqlc.arg('temperature'), sqlc.arg('max_tokens'),
  sqlc.arg('handoff_policy'), sqlc.arg('metadata')
)
ON CONFLICT (slug)
DO UPDATE SET
  name           = EXCLUDED.name,
  system_prompt  = EXCLUDED.system_prompt,
  personality    = EXCLUDED.personality,
  capabilities   = EXCLUDED.capabilities,
  model          = EXCLUDED.model,
  temperature    = EXCLUDED.temperature,
  max_tokens     = EXCLUDED.max_tokens,
  handoff_policy = EXCLUDED.handoff_policy,
  metadata       = EXCLUDED.metadata,
  updated_at     = now()
RETURNING id, slug, name, system_prompt,
          COALESCE(personality, '')::text AS personality,
          capabilities, model, temperature, max_tokens,
          handoff_policy, metadata;

-- name: GetAgentTemplateBySlug :one
SELECT id, slug, name, system_prompt,
       COALESCE(personality, '')::text AS personality,
       capabilities, model, temperature, max_tokens,
       handoff_policy, metadata
FROM agent_templates
WHERE slug = sqlc.arg('slug');

-- name: ListAgentTemplates :many
SELECT id, slug, name, system_prompt,
       COALESCE(personality, '')::text AS personality,
       capabilities, model, temperature, max_tokens,
       handoff_policy, metadata
FROM agent_templates
ORDER BY slug ASC
LIMIT sqlc.arg('result_limit')::int;

-- name: DeleteAgentTemplate :execrows
DELETE FROM agent_templates WHERE slug = sqlc.arg('slug');
