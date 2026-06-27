-- ===========================================================================
-- Code graph — grafo de CÓDIGO del repo (Go-only v1), tablas code_nodes /
-- code_edges / code_index_files (mig 000178). Aislamiento por project_id
-- (single-tenant). El binario domain-mcp parsea el AST client-side y materializa
-- el grafo aquí; code_index_files da incrementalidad por content_hash.
-- ===========================================================================

-- ---------------------------------------------------------------------------
-- Nodes
-- ---------------------------------------------------------------------------

-- name: UpsertNode :one
-- Upsert idempotente por (project_id, qualified_name, kind) sobre nodos activos.
-- Si el nodo existe (no borrado) actualiza ubicación/firma/doc/hash y reabre por
-- si estaba borrado (deleted_at = NULL). El conflict target replica el predicado
-- del índice único parcial de la mig 000178.
INSERT INTO code_nodes
   (project_id, kind, name, qualified_name, file_path, line_start, line_end,
    signature, doc, language, content_hash, metadata)
 VALUES (sqlc.arg('project_id'), sqlc.arg('kind'), sqlc.arg('name'),
         sqlc.arg('qualified_name'), sqlc.arg('file_path'),
         sqlc.arg('line_start'), sqlc.arg('line_end'),
         sqlc.arg('signature'), sqlc.arg('doc'), sqlc.arg('language'),
         sqlc.arg('content_hash'), sqlc.arg('metadata'))
 ON CONFLICT (project_id, qualified_name, kind)
   WHERE deleted_at IS NULL
   DO UPDATE SET
     name         = EXCLUDED.name,
     file_path    = EXCLUDED.file_path,
     line_start   = EXCLUDED.line_start,
     line_end     = EXCLUDED.line_end,
     signature    = EXCLUDED.signature,
     doc          = EXCLUDED.doc,
     language     = EXCLUDED.language,
     content_hash = EXCLUDED.content_hash,
     metadata     = EXCLUDED.metadata,
     deleted_at   = NULL,
     updated_at   = NOW()
 RETURNING id, project_id, kind, name, qualified_name, file_path,
           line_start, line_end, signature, doc, language, content_hash,
           metadata, created_at, updated_at;

-- name: GetNodeByQualified :one
SELECT id, project_id, kind, name, qualified_name, file_path,
       line_start, line_end, signature, doc, language, content_hash,
       metadata, created_at, updated_at
FROM code_nodes
WHERE project_id = sqlc.arg('project_id')
  AND qualified_name = sqlc.arg('qualified_name')
  AND kind = sqlc.arg('kind')
  AND deleted_at IS NULL;

-- name: ListNodesByFile :many
SELECT id, project_id, kind, name, qualified_name, file_path,
       line_start, line_end, signature, doc, language, content_hash,
       metadata, created_at, updated_at
FROM code_nodes
WHERE project_id = sqlc.arg('project_id')
  AND file_path = sqlc.arg('file_path')
  AND deleted_at IS NULL
ORDER BY line_start ASC;

-- name: ListNodesByProject :many
SELECT id, project_id, kind, name, qualified_name, file_path,
       line_start, line_end, signature, doc, language, content_hash,
       metadata, created_at, updated_at
FROM code_nodes
WHERE project_id = sqlc.arg('project_id')
  AND deleted_at IS NULL
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind')::text)
ORDER BY file_path ASC, line_start ASC;

-- name: SearchNodesByName :many
SELECT id, project_id, kind, name, qualified_name, file_path,
       line_start, line_end, signature, doc, language, content_hash,
       metadata, created_at, updated_at
FROM code_nodes
WHERE project_id = sqlc.arg('project_id')
  AND deleted_at IS NULL
  AND (sqlc.narg('kind')::text IS NULL OR kind = sqlc.narg('kind')::text)
  AND (name ILIKE sqlc.arg('pattern') OR qualified_name ILIKE sqlc.arg('pattern'))
ORDER BY name ASC
LIMIT sqlc.arg('result_limit');

-- name: SoftDeleteNodesByFile :execrows
-- Marca como borrados todos los nodos activos de un archivo. Se usa al eliminar
-- un archivo que desapareció del disco (removeFile).
UPDATE code_nodes
SET deleted_at = NOW()
WHERE project_id = sqlc.arg('project_id')
  AND file_path = sqlc.arg('file_path')
  AND deleted_at IS NULL;

-- name: SoftDeleteNodesByFileExcept :many
-- Re-parseo: marca como borrados los nodos activos de un archivo cuyo id NO está
-- en el conjunto de ids recién upserteados (símbolos que ya no existen en el
-- archivo). Los ids retenidos conservan su identidad (no se soft-deletean), por
-- lo que las aristas entrantes y los vínculos memoria/código siguen apuntando al
-- mismo node_id. RETURNING ids para que el caller borre sus aristas colgantes.
UPDATE code_nodes
SET deleted_at = NOW()
WHERE project_id = sqlc.arg('project_id')
  AND file_path = sqlc.arg('file_path')
  AND deleted_at IS NULL
  AND id <> ALL(sqlc.arg('keep_node_ids')::uuid[])
RETURNING id;

-- ---------------------------------------------------------------------------
-- Edges
-- ---------------------------------------------------------------------------

-- name: InsertEdgeIfAbsent :one
-- Inserción idempotente: si ya existe (mismo project/source/target/tipo) NO
-- inserta ni lanza excepción (evita abortar la tx en loops de inserción masiva).
-- RETURNING vacío => no se insertó (ya existía).
INSERT INTO code_edges
   (project_id, source_node_id, target_node_id, edge_type, metadata)
 VALUES (sqlc.arg('project_id'), sqlc.arg('source_node_id'),
         sqlc.arg('target_node_id'), sqlc.arg('edge_type'), sqlc.arg('metadata'))
 ON CONFLICT (project_id, source_node_id, target_node_id, edge_type)
   DO NOTHING
 RETURNING id, project_id, source_node_id, target_node_id, edge_type,
           metadata, created_at;

-- name: ListEdgesBySource :many
SELECT id, project_id, source_node_id, target_node_id, edge_type,
       metadata, created_at
FROM code_edges
WHERE project_id = sqlc.arg('project_id')
  AND source_node_id = sqlc.arg('source_node_id')
  AND (sqlc.narg('edge_type')::text IS NULL OR edge_type = sqlc.narg('edge_type')::text)
ORDER BY created_at DESC;

-- name: ListEdgesByTarget :many
SELECT id, project_id, source_node_id, target_node_id, edge_type,
       metadata, created_at
FROM code_edges
WHERE project_id = sqlc.arg('project_id')
  AND target_node_id = sqlc.arg('target_node_id')
  AND (sqlc.narg('edge_type')::text IS NULL OR edge_type = sqlc.narg('edge_type')::text)
ORDER BY created_at DESC;

-- name: ListEdgesByProject :many
SELECT id, project_id, source_node_id, target_node_id, edge_type,
       metadata, created_at
FROM code_edges
WHERE project_id = sqlc.arg('project_id')
  AND (sqlc.narg('edge_type')::text IS NULL OR edge_type = sqlc.narg('edge_type')::text)
ORDER BY created_at DESC;

-- name: DeleteEdgesBySourceNodes :execrows
-- Borra (hard) todas las aristas salientes de un conjunto de nodos source.
-- Se usa al re-parsear un archivo: primero se limpian las aristas de sus nodos.
DELETE FROM code_edges
WHERE project_id = sqlc.arg('project_id')
  AND source_node_id = ANY(sqlc.arg('source_node_ids')::uuid[]);

-- name: DeleteEdgesByTargetNodes :execrows
-- Borra (hard) todas las aristas ENTRANTES de un conjunto de nodos target. Se usa
-- cuando un nodo deja de existir (re-parseo que lo elimina o archivo borrado):
-- las aristas que apuntaban a él desde OTROS archivos quedarían colgando porque
-- soft-delete es UPDATE y no dispara el ON DELETE CASCADE. Hard-delete evita
-- aristas huérfanas hacia nodos muertos.
DELETE FROM code_edges
WHERE project_id = sqlc.arg('project_id')
  AND target_node_id = ANY(sqlc.arg('target_node_ids')::uuid[]);

-- name: ListEdgesByProjectActive :many
-- Aristas del project cuyos DOS extremos son nodos activos (deleted_at IS NULL).
-- Usado por Path para que el BFS no atraviese nodos muertos via aristas huérfanas
-- que pudieran sobrevivir. Filtra en SQL en vez de en memoria.
SELECT e.id, e.project_id, e.source_node_id, e.target_node_id, e.edge_type,
       e.metadata, e.created_at
FROM code_edges e
JOIN code_nodes s ON s.id = e.source_node_id AND s.deleted_at IS NULL
JOIN code_nodes t ON t.id = e.target_node_id AND t.deleted_at IS NULL
WHERE e.project_id = sqlc.arg('project_id')
  AND (sqlc.narg('edge_type')::text IS NULL OR e.edge_type = sqlc.narg('edge_type')::text)
ORDER BY e.created_at DESC;

-- ---------------------------------------------------------------------------
-- Index files (incrementalidad)
-- ---------------------------------------------------------------------------

-- name: UpsertIndexFile :one
INSERT INTO code_index_files
   (project_id, file_path, content_hash, git_head, node_count, indexed_at)
 VALUES (sqlc.arg('project_id'), sqlc.arg('file_path'), sqlc.arg('content_hash'),
         sqlc.arg('git_head'), sqlc.arg('node_count'), NOW())
 ON CONFLICT (project_id, file_path)
   DO UPDATE SET
     content_hash = EXCLUDED.content_hash,
     git_head     = EXCLUDED.git_head,
     node_count   = EXCLUDED.node_count,
     indexed_at   = NOW()
 RETURNING id, project_id, file_path, content_hash, git_head,
           node_count, indexed_at;

-- name: GetIndexFile :one
SELECT id, project_id, file_path, content_hash, git_head,
       node_count, indexed_at
FROM code_index_files
WHERE project_id = sqlc.arg('project_id')
  AND file_path = sqlc.arg('file_path');

-- name: ListIndexFiles :many
SELECT id, project_id, file_path, content_hash, git_head,
       node_count, indexed_at
FROM code_index_files
WHERE project_id = sqlc.arg('project_id')
ORDER BY file_path ASC;

-- name: DeleteIndexFile :execrows
DELETE FROM code_index_files
WHERE project_id = sqlc.arg('project_id')
  AND file_path = sqlc.arg('file_path');

-- ---------------------------------------------------------------------------
-- Observation <-> Code links (cruce memoria/código, mig 000179)
-- ---------------------------------------------------------------------------

-- name: GetCodeNodeProject :one
-- Devuelve el project del nodo (para validar mismo-project en el cruce).
SELECT id, project_id
FROM code_nodes
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL;

-- name: GetObservationProject :one
-- Devuelve el project de la observation (para validar mismo-project en el cruce).
-- Vive en este paquete para no acoplar el LinkService a observationdb; lee solo
-- las columnas necesarias de knowledge_observations.
SELECT id, project_id
FROM knowledge_observations
WHERE id = sqlc.arg('id')
  AND deleted_at IS NULL;

-- name: InsertObsCodeLinkIfAbsent :one
-- Inserción idempotente del vínculo observation -> code_node. Si ya existe un
-- vínculo activo (mismo project/observation/code_node/tipo) NO inserta ni lanza
-- excepción (ON CONFLICT DO NOTHING sobre el índice único parcial). RETURNING
-- vacío => ya existía.
INSERT INTO knowledge_observation_code_links
   (project_id, observation_id, code_node_id, link_type, note, metadata, created_by)
 VALUES (sqlc.arg('project_id'), sqlc.arg('observation_id'), sqlc.arg('code_node_id'),
         sqlc.arg('link_type'), sqlc.arg('note'), sqlc.arg('metadata'),
         sqlc.arg('created_by'))
 ON CONFLICT (project_id, observation_id, code_node_id, link_type)
   WHERE deleted_at IS NULL
   DO NOTHING
 RETURNING id, project_id, observation_id, code_node_id, link_type,
           note, metadata, created_by, created_at;

-- name: SoftDeleteObsCodeLink :execrows
-- Marca como borrado el vínculo activo (observation, code_node, tipo) del project.
UPDATE knowledge_observation_code_links
SET deleted_at = NOW()
WHERE project_id = sqlc.arg('project_id')
  AND observation_id = sqlc.arg('observation_id')
  AND code_node_id = sqlc.arg('code_node_id')
  AND link_type = sqlc.arg('link_type')
  AND deleted_at IS NULL;

-- name: ListLinksByObservation :many
-- Vínculos activos de una observation (qué nodos de código toca).
SELECT id, project_id, observation_id, code_node_id, link_type,
       note, metadata, created_by, created_at
FROM knowledge_observation_code_links
WHERE observation_id = sqlc.arg('observation_id')
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: ListLinksByCodeNode :many
-- Vínculos activos de un nodo de código (qué memorias lo afectan).
SELECT id, project_id, observation_id, code_node_id, link_type,
       note, metadata, created_by, created_at
FROM knowledge_observation_code_links
WHERE code_node_id = sqlc.arg('code_node_id')
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: ListObservationsByCodeNode :many
-- Decisiones/memorias que afectan a un nodo de código, con su contenido y tipo
-- (JOIN a knowledge_observations). Acotado al project para el aislamiento.
SELECT l.id            AS link_id,
       l.link_type     AS link_type,
       l.note          AS note,
       l.created_at    AS linked_at,
       o.id            AS observation_id,
       o.content       AS content,
       o.observation_type AS observation_type,
       o.created_at    AS observation_created_at
FROM knowledge_observation_code_links l
JOIN knowledge_observations o ON o.id = l.observation_id AND o.deleted_at IS NULL
WHERE l.project_id = sqlc.arg('project_id')
  AND l.code_node_id = sqlc.arg('code_node_id')
  AND l.deleted_at IS NULL
ORDER BY l.created_at DESC;
