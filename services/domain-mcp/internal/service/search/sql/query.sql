-- name: SearchObservations :many
SELECT o.id, o.observation_type, o.content, o.tags, o.project_id, o.created_at,
       ts_rank(o.content_tsv, qry)::float8 AS score
FROM knowledge_observations o, plainto_tsquery('spanish', sqlc.arg('query')) AS qry
WHERE o.deleted_at IS NULL AND o.content_tsv @@ qry
  AND (sqlc.narg('project_ids')::uuid[] IS NULL OR o.project_id = ANY(sqlc.narg('project_ids')::uuid[]))
  AND (sqlc.narg('tags')::text[] IS NULL OR o.tags @> sqlc.narg('tags')::text[])
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR o.created_at >= sqlc.narg('date_from')::timestamptz)
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR o.created_at < sqlc.narg('date_to')::timestamptz)
ORDER BY score DESC
LIMIT sqlc.arg('result_limit');

-- name: SearchPrompts :many
SELECT p.id, p.slug, p.body, p.tags, p.project_id, p.created_at,
       ts_rank(p.body_tsv, qry)::float8 AS score
FROM prompts p, plainto_tsquery('spanish', sqlc.arg('query')) AS qry
WHERE p.deleted_at IS NULL AND p.body_tsv @@ qry
  AND (sqlc.narg('project_ids')::uuid[] IS NULL OR p.project_id = ANY(sqlc.narg('project_ids')::uuid[]))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR p.created_at >= sqlc.narg('date_from')::timestamptz)
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR p.created_at < sqlc.narg('date_to')::timestamptz)
ORDER BY score DESC
LIMIT sqlc.arg('result_limit');

-- name: SearchKnowledgeDocs :many
SELECT kd.id, kd.title, kd.body, kd.project_id, kd.created_at,
       ts_rank(kd.body_tsv, qry)::float8 AS score
FROM knowledge_docs kd, plainto_tsquery('spanish', sqlc.arg('query')) AS qry
WHERE kd.deleted_at IS NULL AND kd.body_tsv @@ qry
  AND (sqlc.narg('project_ids')::uuid[] IS NULL OR kd.project_id = ANY(sqlc.narg('project_ids')::uuid[]))
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR kd.created_at >= sqlc.narg('date_from')::timestamptz)
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR kd.created_at < sqlc.narg('date_to')::timestamptz)
ORDER BY score DESC
LIMIT sqlc.arg('result_limit');
