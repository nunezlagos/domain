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

-- ===========================================================================
-- Memory graph — aristas tipadas y bi-temporales entre knowledge_observations
-- (tabla knowledge_observation_edges, mig 000175). Aislamiento por project_id
-- (single-tenant): el handler resuelve el project vía slug/observation.
-- ===========================================================================

-- name: InsertEdge :one
INSERT INTO knowledge_observation_edges
   (project_id, source_id, target_id, edge_type, origin,
    confidence, valid_from, note, metadata, created_by)
 VALUES (sqlc.arg('project_id'),
         sqlc.arg('source_id'), sqlc.arg('target_id'), sqlc.arg('edge_type'),
         sqlc.arg('origin'), sqlc.arg('confidence'),
         COALESCE(sqlc.narg('valid_from')::timestamptz, NOW()),
         sqlc.arg('note'), sqlc.arg('metadata'), sqlc.arg('created_by'))
 RETURNING id, project_id, source_id, target_id, edge_type,
           origin, confidence, valid_from, valid_to, note, metadata, created_by,
           created_at, updated_at;

-- name: InsertEdgeIfAbsent :one
-- Inserción idempotente para inferencia masiva: si ya existe una arista activa
-- (mismo project/source/target/tipo, no borrada y vigente) NO inserta y NO lanza
-- excepción (evita abortar la tx con UniqueViolation/25P02 dentro del loop).
-- RETURNING vacío => no se insertó (ya existía). El conflict target replica el
-- predicado del índice único parcial de la mig 000175.
INSERT INTO knowledge_observation_edges
   (project_id, source_id, target_id, edge_type, origin,
    confidence, valid_from, note, metadata, created_by)
 VALUES (sqlc.arg('project_id'),
         sqlc.arg('source_id'), sqlc.arg('target_id'), sqlc.arg('edge_type'),
         sqlc.arg('origin'), sqlc.arg('confidence'),
         COALESCE(sqlc.narg('valid_from')::timestamptz, NOW()),
         sqlc.arg('note'), sqlc.arg('metadata'), sqlc.arg('created_by'))
 ON CONFLICT (project_id, source_id, target_id, edge_type)
   WHERE deleted_at IS NULL AND valid_to IS NULL
   DO NOTHING
 RETURNING id, project_id, source_id, target_id, edge_type,
           origin, confidence, valid_from, valid_to, note, metadata, created_by,
           created_at, updated_at;

-- name: GetEdge :one
SELECT id, project_id, source_id, target_id, edge_type,
       origin, confidence, valid_from, valid_to, note, metadata, created_by,
       created_at, updated_at
FROM knowledge_observation_edges
WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: ListEdgesBySource :many
SELECT id, project_id, source_id, target_id, edge_type,
       origin, confidence, valid_from, valid_to, note, metadata, created_by,
       created_at, updated_at
FROM knowledge_observation_edges
WHERE project_id = sqlc.arg('project_id')
  AND source_id = sqlc.arg('source_id')
  AND deleted_at IS NULL
  AND valid_to IS NULL
  AND (sqlc.narg('edge_type')::text IS NULL OR edge_type = sqlc.narg('edge_type')::text)
ORDER BY created_at DESC;

-- name: ListEdgesByTarget :many
SELECT id, project_id, source_id, target_id, edge_type,
       origin, confidence, valid_from, valid_to, note, metadata, created_by,
       created_at, updated_at
FROM knowledge_observation_edges
WHERE project_id = sqlc.arg('project_id')
  AND target_id = sqlc.arg('target_id')
  AND deleted_at IS NULL
  AND valid_to IS NULL
  AND (sqlc.narg('edge_type')::text IS NULL OR edge_type = sqlc.narg('edge_type')::text)
ORDER BY created_at DESC;

-- name: ListEdgesByProject :many
SELECT id, project_id, source_id, target_id, edge_type,
       origin, confidence, valid_from, valid_to, note, metadata, created_by,
       created_at, updated_at
FROM knowledge_observation_edges
WHERE project_id = sqlc.arg('project_id')
  AND deleted_at IS NULL
  AND valid_to IS NULL
  AND (sqlc.narg('edge_type')::text IS NULL OR edge_type = sqlc.narg('edge_type')::text)
ORDER BY created_at DESC;

-- name: SoftDeleteEdge :execrows
-- Borrado por id (single-tenant): 0 filas si no existe o ya fue borrada; el
-- service lo mapea a ErrEdgeNotFound. Mismo patrón que SoftDeleteObservation.
UPDATE knowledge_observation_edges
SET deleted_at = NOW()
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL;

-- name: CloseEdgeValidTo :execrows
UPDATE knowledge_observation_edges
SET valid_to = NOW()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL AND valid_to IS NULL;

-- name: FindEdgeCandidatesByEmbedding :many
-- Candidatos a relacionar con source_id: top-K observations del mismo project
-- por similitud coseno, excluyendo la propia source y las que ya tienen arista
-- activa source->target. score = 1 - distancia coseno (1.0 = idénticas).
SELECT o.id, o.project_id, o.created_by, o.session_id,
       o.content, o.observation_type, o.tags, o.metadata,
       o.created_at, o.updated_at,
       (1 - (o.embedding <=> sqlc.arg('embedding')::text::vector))::float8 AS score
FROM knowledge_observations o
WHERE o.project_id = sqlc.arg('project_id')
  AND o.deleted_at IS NULL
  AND o.embedding IS NOT NULL
  AND o.id <> sqlc.arg('source_id')
  AND NOT EXISTS (
    SELECT 1 FROM knowledge_observation_edges e
    WHERE e.project_id = o.project_id
      AND e.source_id = sqlc.arg('source_id')
      AND e.target_id = o.id
      AND e.deleted_at IS NULL
      AND e.valid_to IS NULL
  )
ORDER BY o.embedding <=> sqlc.arg('embedding')::text::vector ASC
LIMIT sqlc.arg('result_limit');

-- name: FindCandidatePairsBySignals :many
-- Pares candidatos a relacionar SIN embeddings, por señales baratas:
--   co-sesión (mismo session_id), solapamiento de tags (intersección de arrays),
--   y solapamiento léxico (content_tsv de a contra ts_query del content de b vía
--   plainto_tsquery en español). Devuelve pares dirigidos a.id (source) -> b.id
--   (target) con a.id < b.id para evitar duplicados (i,j)/(j,i), excluyendo pares
--   que ya tienen una arista activa en CUALQUIER dirección y tipo. Opcionalmente
--   ancla a una observation (sqlc.narg('anchor_id')): solo pares que la incluyan.
-- Cada señal suma a un score heurístico [0..1] usado para ordenar los candidatos
-- más prometedores primero; el LLM decide el tipo final.
WITH obs AS (
  SELECT ko.id, ko.session_id, ko.content, ko.observation_type, ko.tags, ko.content_tsv, ko.created_at
  FROM knowledge_observations ko
  WHERE ko.project_id = sqlc.arg('project_id')
    AND ko.deleted_at IS NULL
  ORDER BY ko.created_at DESC
  LIMIT sqlc.arg('scan_limit')
),
pairs AS (
  SELECT
    a.id            AS source_id,
    b.id            AS target_id,
    a.content       AS source_content,
    b.content       AS target_content,
    a.observation_type AS source_type,
    b.observation_type AS target_type,
    a.tags          AS source_tags,
    b.tags          AS target_tags,
    (a.session_id IS NOT NULL AND a.session_id = b.session_id) AS same_session,
    COALESCE(cardinality(ARRAY(SELECT unnest(a.tags) INTERSECT SELECT unnest(b.tags))), 0)::int AS shared_tags,
    (a.content_tsv @@ plainto_tsquery('spanish', b.content)
       OR b.content_tsv @@ plainto_tsquery('spanish', a.content)) AS lexical_overlap,
    ts_rank(a.content_tsv, plainto_tsquery('spanish', b.content))::float8 AS lexical_rank
  FROM obs a
  JOIN obs b ON a.id < b.id
  WHERE
    (sqlc.narg('anchor_id')::uuid IS NULL
       OR a.id = sqlc.narg('anchor_id')::uuid
       OR b.id = sqlc.narg('anchor_id')::uuid)
    AND NOT EXISTS (
      SELECT 1 FROM knowledge_observation_edges e
      WHERE e.project_id = sqlc.arg('project_id')
        AND e.deleted_at IS NULL
        AND e.valid_to IS NULL
        AND ((e.source_id = a.id AND e.target_id = b.id)
          OR (e.source_id = b.id AND e.target_id = a.id))
    )
)
SELECT
  source_id, target_id, source_content, target_content,
  source_type, target_type, source_tags, target_tags,
  same_session, shared_tags, lexical_overlap,
  LEAST(1.0,
    (CASE WHEN same_session THEN 0.34 ELSE 0 END)
    + LEAST(0.33, shared_tags * 0.17)
    + (CASE WHEN lexical_overlap THEN 0.33 + LEAST(0.0, lexical_rank) ELSE 0 END)
  )::float8 AS signal_score
FROM pairs
WHERE same_session OR shared_tags > 0 OR lexical_overlap
ORDER BY signal_score DESC, source_id, target_id
LIMIT sqlc.arg('result_limit');
