-- name: GetAnchorObservation :one
SELECT created_at, project_id, content
FROM knowledge_observations
WHERE id = @observation_id AND deleted_at IS NULL;

-- name: ListObservations :many
SELECT id, observation_type, content, created_at
FROM knowledge_observations
WHERE deleted_at IS NULL
ORDER BY created_at DESC
LIMIT @lim;

-- name: ListObservationsByProject :many
SELECT id, observation_type, content, created_at
FROM knowledge_observations
WHERE project_id = @project_id AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT @lim;

-- name: ListPrompts :many
SELECT id, slug, body, created_at
FROM prompts
WHERE is_active = true AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT @lim;

-- name: ListPromptsByProject :many
SELECT id, slug, body, created_at
FROM prompts
WHERE project_id = @project_id AND is_active = true AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT @lim;

-- name: ListEntriesBefore :many
SELECT 'observation'::text AS kind, ko.id, ko.observation_type, ko.content, ko.created_at
FROM knowledge_observations ko
WHERE ko.project_id = @project_id AND ko.created_at < @ts AND ko.deleted_at IS NULL
UNION ALL
SELECT 'prompt'::text AS kind, p.id, p.slug, p.body, p.created_at
FROM prompts p
WHERE p.project_id = @project_id AND p.created_at < @ts AND p.is_active = true AND p.deleted_at IS NULL
ORDER BY created_at DESC
LIMIT @lim;

-- name: ListEntriesAfter :many
SELECT 'observation'::text AS kind, ko.id, ko.observation_type, ko.content, ko.created_at
FROM knowledge_observations ko
WHERE ko.project_id = @project_id AND ko.created_at > @ts AND ko.deleted_at IS NULL
UNION ALL
SELECT 'prompt'::text AS kind, p.id, p.slug, p.body, p.created_at
FROM prompts p
WHERE p.project_id = @project_id AND p.created_at > @ts AND p.is_active = true AND p.deleted_at IS NULL
ORDER BY created_at ASC
LIMIT @lim;
