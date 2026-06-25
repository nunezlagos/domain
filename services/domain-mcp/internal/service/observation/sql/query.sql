-- name: InsertObservation :one
INSERT INTO knowledge_observations
   (project_id, created_by, session_id, content,
    embedding, observation_type, tags, metadata, content_hash)
 VALUES (sqlc.arg('project_id'), sqlc.arg('created_by'), sqlc.arg('session_id'),
         sqlc.arg('content'),
         CASE WHEN sqlc.arg('embedding')::text = '[]' THEN NULL ELSE sqlc.arg('embedding')::text::vector END,
         sqlc.arg('observation_type'), sqlc.arg('tags'), sqlc.arg('metadata'),
         sqlc.arg('content_hash'))
 RETURNING id, project_id, created_by, session_id,
           content, observation_type, tags, metadata, created_at, updated_at;

-- name: GetObservation :one
SELECT id, project_id, created_by, session_id,
       content, observation_type, tags, metadata, created_at, updated_at
FROM knowledge_observations WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: ListObservations :many
SELECT id, project_id, created_by, session_id,
       content, observation_type, tags, metadata, created_at, updated_at
FROM knowledge_observations
WHERE project_id = sqlc.arg('project_id') AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT sqlc.arg('result_limit');

-- name: SoftDeleteObservation :execrows
UPDATE knowledge_observations SET deleted_at = NOW() WHERE id = sqlc.arg('id') AND deleted_at IS NULL;
