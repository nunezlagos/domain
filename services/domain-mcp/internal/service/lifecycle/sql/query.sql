-- lifecycle: erasure.go

-- name: GetUserIsErased :one
SELECT id, is_erased FROM users WHERE id = sqlc.arg('id');

-- name: EraseUserPII :execrows
UPDATE users SET
    email    = 'erased+' || id::text || '@example.invalid',
    rut      = NULL,
    name     = NULL,
    is_erased = TRUE,
    erased_at = NOW()
WHERE id = sqlc.arg('id');

-- name: AnonymizeObservations :execrows
UPDATE knowledge_observations SET created_by = NULL WHERE created_by = sqlc.arg('user_id');

-- name: AnonymizePrompts :execrows
UPDATE prompts SET created_by = NULL WHERE created_by = sqlc.arg('user_id');

-- name: AnonymizeKnowledgeDocs :execrows
UPDATE knowledge_docs SET created_by = NULL WHERE created_by = sqlc.arg('user_id');

-- name: AnonymizeAgentRuns :execrows
UPDATE agent_runs SET user_id = NULL WHERE user_id = sqlc.arg('user_id');

-- name: RevokeUserAPIKeys :execrows
UPDATE auth_api_keys SET revoked_at = NOW()
WHERE user_id = sqlc.arg('user_id') AND revoked_at IS NULL;

-- lifecycle: service.go (ExportUserData)

-- name: GetUserForExport :one
SELECT id, email, name, role, created_at, updated_at, deleted_at
FROM users WHERE id = sqlc.arg('id');

-- name: ListProjectsForExport :many
SELECT id, name, slug, description, created_at
FROM projects
ORDER BY created_at DESC;

-- name: ListObservationsByCreator :many
SELECT id, project_id, content, observation_type, tags, metadata, created_at
FROM knowledge_observations
WHERE created_by = sqlc.arg('created_by') AND deleted_at IS NULL;

-- name: ListPromptsByCreator :many
SELECT id, project_id, slug, version, body, is_active, created_at
FROM prompts
WHERE created_by = sqlc.arg('created_by') AND deleted_at IS NULL;

-- name: ListKnowledgeDocsByCreator :many
SELECT id, project_id, title, source, source_url, tags, created_at
FROM knowledge_docs
WHERE created_by = sqlc.arg('created_by') AND deleted_at IS NULL;

-- name: ListAgentRunsByUser :many
SELECT id, agent_id, status, inputs, outputs, tokens_input, tokens_output,
       cost_usd, iterations, started_at, finished_at
FROM agent_runs WHERE user_id = sqlc.arg('user_id');

-- name: ListAPIKeysByUser :many
SELECT id, name, key_prefix, last_used_at, expires_at, revoked_at, created_at
FROM auth_api_keys WHERE user_id = sqlc.arg('user_id');

-- name: ListAuditEntriesByActor :many
SELECT id, action, entity_type, entity_id, new_values, occurred_at
FROM audit_log WHERE actor_id = sqlc.arg('actor_id')
ORDER BY occurred_at DESC LIMIT 5000;
