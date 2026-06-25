-- name: InsertDoc :one
INSERT INTO knowledge_docs (
    project_id, created_by, title, body, source, source_url, tags, metadata
) VALUES (
    sqlc.arg('project_id'), sqlc.arg('created_by'), sqlc.arg('title'), sqlc.arg('body'),
    sqlc.arg('source'), sqlc.arg('source_url'), sqlc.arg('tags'), sqlc.arg('metadata')
)
RETURNING id, project_id, created_by, title, body, source,
          source_url,
          tags, metadata, has_attachments, created_at, updated_at;

-- name: InsertChunk :one
INSERT INTO knowledge_chunks (knowledge_doc_id, chunk_index, content, embedding)
VALUES (sqlc.arg('knowledge_doc_id'), sqlc.arg('chunk_index'), sqlc.arg('content'), sqlc.arg('embedding')::vector)
RETURNING id, knowledge_doc_id, chunk_index, content, created_at;

-- name: GetDoc :one
SELECT id, project_id, created_by, title, body, source,
       source_url,
       tags, metadata, has_attachments, created_at, updated_at
FROM knowledge_docs
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: GetChunks :many
SELECT id, knowledge_doc_id, chunk_index, content, created_at
FROM knowledge_chunks
WHERE knowledge_doc_id = sqlc.arg('knowledge_doc_id')
ORDER BY chunk_index ASC;

-- name: SearchHybrid :many
WITH bm25 AS (
  SELECT c.id, ROW_NUMBER() OVER (ORDER BY ts_rank(c.content_tsv, q) DESC) AS r
  FROM knowledge_chunks c
  JOIN knowledge_docs d ON d.id = c.knowledge_doc_id
       AND d.deleted_at IS NULL,
       plainto_tsquery('spanish', sqlc.arg('query_text')) AS q
  WHERE c.content_tsv @@ q
  LIMIT sqlc.arg('candidates')::int
),
vec AS (
  SELECT c.id, ROW_NUMBER() OVER (ORDER BY c.embedding <=> sqlc.arg('query_vec')::vector ASC) AS r
  FROM knowledge_chunks c
  JOIN knowledge_docs d ON d.id = c.knowledge_doc_id
       AND d.deleted_at IS NULL
  WHERE c.embedding IS NOT NULL
  LIMIT sqlc.arg('candidates')::int
),
fused AS (
  SELECT COALESCE(bm25.id, vec.id) AS id,
         COALESCE(1.0 / (sqlc.arg('rrf_k')::int + bm25.r), 0) + COALESCE(1.0 / (sqlc.arg('rrf_k')::int + vec.r), 0) AS score
  FROM bm25 FULL OUTER JOIN vec ON bm25.id = vec.id
)
SELECT c.id, c.knowledge_doc_id, c.chunk_index, d.title, c.content,
       d.project_id, c.created_at, fused.score::float8 AS score
FROM fused
JOIN knowledge_chunks c ON c.id = fused.id
JOIN knowledge_docs d ON d.id = c.knowledge_doc_id
ORDER BY fused.score DESC
LIMIT sqlc.arg('result_limit')::int;

-- name: SearchBm25 :many
SELECT c.id, c.knowledge_doc_id, c.chunk_index, d.title, c.content,
       d.project_id, c.created_at,
       ts_rank(c.content_tsv, q)::float8 AS score
FROM knowledge_chunks c
JOIN knowledge_docs d ON d.id = c.knowledge_doc_id
     AND d.deleted_at IS NULL,
     plainto_tsquery('spanish', sqlc.arg('query_text')) AS q
WHERE c.content_tsv @@ q
ORDER BY score DESC
LIMIT sqlc.arg('result_limit')::int;

-- name: SoftDeleteDoc :execrows
UPDATE knowledge_docs SET deleted_at = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: ListDocsByProject :many
SELECT id, project_id, created_by, title, body, source,
       source_url,
       tags, metadata, has_attachments, created_at, updated_at
FROM knowledge_docs
WHERE project_id = sqlc.arg('project_id') AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT sqlc.arg('result_limit')::int;
