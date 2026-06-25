-- name: InsertPrompt :one
INSERT INTO prompt_captured (user_id, project_id, content, client_kind, model, char_count, estimated_tokens_in)
VALUES (sqlc.arg('user_id'), sqlc.arg('project_id'), sqlc.arg('content'),
        NULLIF(sqlc.arg('client_kind'), ''), NULLIF(sqlc.arg('model'), ''),
        sqlc.arg('char_count'), sqlc.arg('estimated_tokens_in'))
RETURNING id, user_id, project_id, content,
          COALESCE(client_kind,'')::text AS client_kind,
          COALESCE(model,'')::text AS model,
          char_count, response_chars, estimated_tokens_in, estimated_tokens_out,
          captured_at, turn_completed_at;

-- name: CompleteTurn :one
UPDATE prompt_captured
SET response_chars = sqlc.arg('response_chars'),
    estimated_tokens_out = sqlc.arg('estimated_tokens_out'),
    model = COALESCE(NULLIF(sqlc.arg('model'), ''), model),
    turn_completed_at = NOW()
WHERE id = sqlc.arg('id')
RETURNING id, user_id, project_id, content,
          COALESCE(client_kind,'')::text AS client_kind,
          COALESCE(model,'')::text AS model,
          char_count, response_chars, estimated_tokens_in, estimated_tokens_out,
          captured_at, turn_completed_at;

-- name: GetPrompt :one
SELECT id, user_id, project_id, content,
       COALESCE(client_kind,'')::text AS client_kind,
       COALESCE(model,'')::text AS model,
       char_count, response_chars, estimated_tokens_in, estimated_tokens_out,
       captured_at, turn_completed_at
FROM prompt_captured WHERE id = sqlc.arg('id');

-- name: CountPrompts :one
SELECT COUNT(*) FROM prompt_captured
WHERE (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id')::uuid)
  AND (sqlc.narg('user_id')::uuid IS NULL OR user_id = sqlc.narg('user_id')::uuid);

-- name: ListPrompts :many
SELECT id, user_id, project_id, content,
       COALESCE(client_kind,'')::text AS client_kind,
       COALESCE(model,'')::text AS model,
       char_count, response_chars, estimated_tokens_in, estimated_tokens_out,
       captured_at, turn_completed_at
FROM prompt_captured
WHERE (sqlc.narg('project_id')::uuid IS NULL OR project_id = sqlc.narg('project_id')::uuid)
  AND (sqlc.narg('user_id')::uuid IS NULL OR user_id = sqlc.narg('user_id')::uuid)
ORDER BY captured_at DESC
LIMIT sqlc.arg('result_limit')::int
OFFSET sqlc.arg('result_offset')::int;

-- name: SummarizeByProject :one
SELECT COUNT(*)::int AS turns,
       COALESCE(SUM(estimated_tokens_in),0)::bigint AS estimated_tokens_in,
       COALESCE(SUM(estimated_tokens_out),0)::bigint AS estimated_tokens_out,
       COALESCE(SUM(char_count + response_chars),0)::bigint AS total_chars
FROM prompt_captured WHERE project_id = sqlc.arg('project_id');
