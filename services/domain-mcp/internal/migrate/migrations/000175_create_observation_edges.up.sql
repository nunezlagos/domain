-- migration: 000175_create_observation_edges
-- author: NunezLagos
-- issue: REQ-knowledge-graph
-- description: grafo de relaciones explicitas y tipadas entre knowledge_observations (memory graph)
-- breaking: no
-- estimated_duration: unknown
--
-- detalle: grafo de relaciones explícitas y tipadas entre knowledge_observations
--   (memorias). Hasta ahora las relaciones eran implícitas (project_id, session_id,
--   metadata). Esta tabla las hace explícitas: una decisión que revierte a otra
--   (supersedes), dos que se contradicen (contradicts), linaje causal (derived_from /
--   depends_on) o relación genérica (relates_to, típicamente auto-inferida por embedding).
--
--   Modelo BI-TEMPORAL (estilo Graphiti/Zep):
--     - valid_from / valid_to : valid time — desde/hasta cuándo la relación es verdadera
--       en el dominio. valid_to NULL = vigente. Permite "¿qué decisión estaba vigente en X?".
--     - created_at            : transaction time — cuándo el sistema registró la arista.
--
--   DIRECCIONALIDAD (source -> target):
--     supersedes    : source reemplaza/revierte a target (target queda obsoleto).
--     derived_from  : source se deriva de target.
--     depends_on    : source depende de target.
--     contradicts   : source contradice a target (semánticamente simétrico; se guarda dirigido).
--     relates_to    : relación genérica (semánticamente simétrico).
--
--   org isolation: el modelo org fue retirado (mig 000142 dropeó organization_id de
--   todas las tablas; mig 000143 dropeó la tabla organizations). El sistema es
--   single-tenant: el aislamiento es por project_id, igual que knowledge_observations.
--   Por eso esta tabla NO lleva organization_id ni FK a organizations.
-- nota: tabla nueva, sin backfill.

CREATE TABLE IF NOT EXISTS knowledge_observation_edges (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  source_id       UUID NOT NULL REFERENCES knowledge_observations(id) ON DELETE CASCADE,
  target_id       UUID NOT NULL REFERENCES knowledge_observations(id) ON DELETE CASCADE,

  edge_type       VARCHAR(30) NOT NULL
    CONSTRAINT knowledge_observation_edges_type_check
    CHECK (edge_type IN ('supersedes','contradicts','derived_from','depends_on','relates_to')),

  -- cómo nació la arista: 'manual' (LLM/usuario la afirmó) o 'inferred' (pgvector/heurística)
  origin          VARCHAR(20) NOT NULL DEFAULT 'manual'
    CONSTRAINT knowledge_observation_edges_origin_check
    CHECK (origin IN ('manual','inferred')),
  -- confianza 0..1. manual = 1.0; inferred = score de similitud/heurística.
  confidence      REAL NOT NULL DEFAULT 1.0
    CONSTRAINT knowledge_observation_edges_confidence_check
    CHECK (confidence >= 0 AND confidence <= 1),

  -- bi-temporal: valid time
  valid_from      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  valid_to        TIMESTAMPTZ,  -- NULL = vigente

  note            TEXT,
  metadata        JSONB NOT NULL DEFAULT '{}',
  created_by      UUID REFERENCES users(id) ON DELETE SET NULL,

  -- transaction time
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ,

  CONSTRAINT knowledge_observation_edges_no_self CHECK (source_id <> target_id)
);

CREATE TRIGGER set_updated_at_knowledge_observation_edges
  BEFORE UPDATE ON knowledge_observation_edges
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- una sola arista activa (vigente, no borrada) por (par dirigido + tipo).
-- versiones históricas (valid_to IS NOT NULL) o borradas no chocan.
CREATE UNIQUE INDEX knowledge_observation_edges_active_uniq
  ON knowledge_observation_edges (project_id, source_id, target_id, edge_type)
  WHERE deleted_at IS NULL AND valid_to IS NULL;

-- traversal forward (vecinos salientes de una observation)
CREATE INDEX knowledge_observation_edges_source_idx
  ON knowledge_observation_edges (project_id, source_id)
  WHERE deleted_at IS NULL;

-- traversal backward (quién apunta a esta observation)
CREATE INDEX knowledge_observation_edges_target_idx
  ON knowledge_observation_edges (project_id, target_id)
  WHERE deleted_at IS NULL;

-- filtrar por tipo + vigencia (ej. "todos los supersedes vigentes del proyecto")
CREATE INDEX knowledge_observation_edges_type_valid_idx
  ON knowledge_observation_edges (project_id, edge_type)
  WHERE deleted_at IS NULL AND valid_to IS NULL;

-- Grants consistentes con el resto de tablas (org isolation a nivel de app).
GRANT SELECT, INSERT, UPDATE, DELETE ON knowledge_observation_edges TO app_user;
GRANT ALL ON knowledge_observation_edges TO app_admin;
