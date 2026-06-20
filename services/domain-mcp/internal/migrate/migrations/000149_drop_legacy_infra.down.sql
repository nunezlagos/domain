-- migration: drop_legacy_infra (down)
-- author: mnunez@saargo.com
-- issue: REQ-42.3 (taxonomía de naming — drop legacy/infra)
-- description: reverse de 000149. Recrea las 8 tablas legacy/infra con su shape
--   POST-FASE-C (sin organization_id, con status), índices/triggers/función/view
--   donde aplique, y re-agrega las columnas session_id (FK nullable ON DELETE
--   SET NULL) a captured_prompts y verifications. Incluye el esqueleto de
--   saga_compensation_log. NO recrea sabotage_records (se preserva como
--   tdd_sabotage_records, fuera del drop). Solo roundtrip; NO restaura datos.
-- breaking: false (down de roundtrip; sin restauración real de datos)
-- estimated_duration: <1s
--
-- Revertir: recrear las 8 tablas legacy/infra con su shape POST-FASE-C.
-- IMPORTANTE: NO se recrea el shape literal de las migraciones originales —
-- esas declaraban `organization_id NOT NULL REFERENCES organizations(id)`, pero
-- organizations ya no existe (dropeada en 000143) y organization_id fue removido
-- en 000141/142. Además 000120 agregó la columna `status` a todas. Por eso el
-- down recrea el estado consistente con las migraciones previas a esta HU.
-- Los DATOS no se restauran (las tablas estaban vacías; restore real: pgBackRest).
-- En roundtrip (DB fresca) deja la DB en estado consistente para re-aplicar 000149.

BEGIN;

-- ===== sessions (000007 + 000085 RLS + 000120 status) =====
CREATE TABLE IF NOT EXISTS sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title VARCHAR(500),
  summary TEXT,
  summary_tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', coalesce(summary, ''))) STORED,
  tags TEXT[] NOT NULL DEFAULT '{}',
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ended_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);
CREATE TRIGGER set_updated_at_sessions
  BEFORE UPDATE ON sessions
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX IF NOT EXISTS sessions_user_started_idx ON sessions (user_id, started_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS sessions_project_idx ON sessions (project_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS sessions_summary_tsv_idx ON sessions USING GIN (summary_tsv);
CREATE INDEX IF NOT EXISTS sessions_status_idx ON sessions (status);

-- ===== model_registry (000034 + 000120 status) =====
CREATE TABLE IF NOT EXISTS model_registry (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider VARCHAR(50) NOT NULL
    CHECK (provider IN ('anthropic','openai','google','ollama','voyage')),
  model VARCHAR(100) NOT NULL,
  display_name VARCHAR(255) NOT NULL,
  modality VARCHAR(20) NOT NULL DEFAULT 'completion'
    CHECK (modality IN ('completion','embedding','image','audio')),
  context_size INT,
  input_per_million NUMERIC(10,4),
  output_per_million NUMERIC(10,4),
  embedding_dimensions INT,
  is_active BOOLEAN NOT NULL DEFAULT true,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  deprecated_at TIMESTAMPTZ,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (provider, model)
);
CREATE TRIGGER set_updated_at_model_registry
  BEFORE UPDATE ON model_registry
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE INDEX IF NOT EXISTS model_registry_provider_active_idx
  ON model_registry (provider, model) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS model_registry_status_idx ON model_registry (status);
GRANT SELECT ON model_registry TO app_user, app_readonly;
GRANT ALL ON model_registry TO app_admin;

-- ===== entity_state_transitions (000060 + 000120 status) =====
CREATE TABLE IF NOT EXISTS entity_state_transitions (
  id BIGSERIAL PRIMARY KEY,
  entity_kind VARCHAR(30) NOT NULL
    CHECK (entity_kind IN ('intake','req','hu','sync_state','proposal','design','task')),
  entity_id UUID NOT NULL,
  from_state VARCHAR(40),
  to_state VARCHAR(40) NOT NULL,
  actor_kind VARCHAR(20) NOT NULL
    CHECK (actor_kind IN ('user','agent','system','external')),
  actor_id UUID,
  actor_name VARCHAR(120),
  reason TEXT,
  context JSONB,
  tx_id UUID,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS entity_state_transitions_entity_idx
  ON entity_state_transitions (entity_kind, entity_id, occurred_at);
CREATE INDEX IF NOT EXISTS entity_state_transitions_to_state_idx
  ON entity_state_transitions (entity_kind, to_state, occurred_at);
CREATE INDEX IF NOT EXISTS entity_state_transitions_actor_idx
  ON entity_state_transitions (actor_id) WHERE actor_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS entity_state_transitions_status_idx
  ON entity_state_transitions (status);
CREATE OR REPLACE FUNCTION entity_state_transitions_immutable() RETURNS TRIGGER AS $$
BEGIN
  RAISE EXCEPTION 'entity_state_transitions is append-only (op=%)', TG_OP;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER entity_state_transitions_no_update
  BEFORE UPDATE OR DELETE ON entity_state_transitions
  FOR EACH ROW EXECUTE FUNCTION entity_state_transitions_immutable();
CREATE OR REPLACE VIEW v_stuck_entities AS
SELECT
  entity_kind,
  entity_id,
  to_state AS current_state,
  occurred_at AS last_transition_at,
  EXTRACT(EPOCH FROM (now() - occurred_at)) / 3600 AS hours_in_state
FROM (
  SELECT DISTINCT ON (entity_kind, entity_id)
    entity_kind, entity_id, to_state, occurred_at
  FROM entity_state_transitions
  ORDER BY entity_kind, entity_id, occurred_at DESC
) latest
WHERE to_state NOT IN ('done','archived','rejected','committed','expired','abandoned');
GRANT SELECT, INSERT ON entity_state_transitions TO app_user;
GRANT USAGE, SELECT ON SEQUENCE entity_state_transitions_id_seq TO app_user;
GRANT SELECT ON v_stuck_entities TO app_user;

-- ===== system_state (000074 + 000120 status) =====
CREATE TABLE IF NOT EXISTS system_state (
  key VARCHAR(100) PRIMARY KEY,
  value JSONB NOT NULL DEFAULT '{}',
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS system_state_status_idx ON system_state (status);
GRANT SELECT, INSERT, UPDATE, DELETE ON system_state TO app_user;
DO $g$ BEGIN
  EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON system_state TO app_admin';
EXCEPTION WHEN OTHERS THEN NULL;
END $g$;
DO $g$ BEGIN
  EXECUTE 'GRANT SELECT ON system_state TO app_readonly';
EXCEPTION WHEN OTHERS THEN NULL;
END $g$;

-- ===== saga_compensation_log (cluster saga/infra + status) =====
-- Recreación FIEL del shape real (verificado vía \d), al mismo nivel de
-- fidelidad que el resto del down. PK bigserial, FK saliente run_id → flow_runs
-- (NOT NULL, ON DELETE CASCADE), columnas de auditoría completas, los 2 índices
-- introspectados (saga_compensation_log_status_idx, saga_log_run_idx) y el
-- trigger set_updated_at. La tabla original tenía 0 filas; NO restaura datos.
-- NOTA: sabotage_records NO se recrea aquí: se preserva como tdd_sabotage_records
-- (rename en 000151), no es parte del drop de este lote.
CREATE TABLE IF NOT EXISTS saga_compensation_log (
  id BIGSERIAL PRIMARY KEY,
  run_id UUID NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
  original_step VARCHAR(120) NOT NULL,
  compensate_ran VARCHAR(120) NOT NULL,
  success BOOLEAN NOT NULL,
  error TEXT,
  payload JSONB,
  executed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  status TEXT NOT NULL DEFAULT 'active'
);
CREATE INDEX IF NOT EXISTS saga_compensation_log_status_idx ON saga_compensation_log (status);
CREATE INDEX IF NOT EXISTS saga_log_run_idx ON saga_compensation_log (run_id, executed_at);
CREATE TRIGGER trg_set_updated_at
  BEFORE UPDATE ON saga_compensation_log
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
GRANT SELECT, INSERT, UPDATE, DELETE ON saga_compensation_log TO app_user;
GRANT USAGE, SELECT ON SEQUENCE saga_compensation_log_id_seq TO app_user;

-- ===== runtime_configs (000041 + 000120 status) =====
CREATE TABLE IF NOT EXISTS runtime_configs (
  key             VARCHAR(80) PRIMARY KEY,
  value           JSONB NOT NULL,
  description     TEXT NOT NULL DEFAULT '',
  is_hot_reloadable BOOLEAN NOT NULL DEFAULT TRUE,
  status          VARCHAR(20) NOT NULL DEFAULT 'active',
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by      UUID
);
CREATE INDEX IF NOT EXISTS runtime_configs_updated_at_idx ON runtime_configs(updated_at DESC);
CREATE INDEX IF NOT EXISTS runtime_configs_status_idx ON runtime_configs (status);
GRANT SELECT ON runtime_configs TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON runtime_configs TO app_admin;
GRANT SELECT ON runtime_configs TO app_readonly;

-- ===== dead_letter_queue (000079 + 000120 status) =====
CREATE TABLE IF NOT EXISTS dead_letter_queue (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  flow_run_id UUID REFERENCES flow_runs(id) ON DELETE CASCADE,
  flow_slug VARCHAR(120) NOT NULL,
  step_key VARCHAR(120) NOT NULL,
  error TEXT NOT NULL,
  errors JSONB NOT NULL DEFAULT '[]',
  retry_count INT NOT NULL DEFAULT 0,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  failed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  resolved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- domain-lint-ignore-next: require-concurrent-index-creation
-- reason: tabla nueva sin tráfico (recreación en rollback)
CREATE INDEX IF NOT EXISTS dead_letter_queue_pending_idx
  ON dead_letter_queue (failed_at DESC) WHERE resolved_at IS NULL;
CREATE INDEX IF NOT EXISTS dead_letter_queue_status_idx ON dead_letter_queue (status);
GRANT SELECT, INSERT, UPDATE, DELETE ON dead_letter_queue TO app_user;

-- ===== idempotency_keys (000036 + 000120 status) =====
CREATE TABLE IF NOT EXISTS idempotency_keys (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  key VARCHAR(255) NOT NULL,
  request_method VARCHAR(10) NOT NULL,
  request_path VARCHAR(500) NOT NULL,
  request_body_hash BYTEA NOT NULL,
  response_status SMALLINT NOT NULL,
  response_headers JSONB NOT NULL DEFAULT '{}',
  response_body BYTEA,
  status VARCHAR(20) NOT NULL DEFAULT 'active',
  expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '24 hours'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (key)
);
CREATE INDEX IF NOT EXISTS idempotency_keys_expires_idx ON idempotency_keys (expires_at);
CREATE INDEX IF NOT EXISTS idempotency_keys_status_idx ON idempotency_keys (status);
GRANT SELECT, INSERT, DELETE ON idempotency_keys TO app_user;
GRANT USAGE, SELECT ON SEQUENCE idempotency_keys_id_seq TO app_user;
GRANT ALL ON idempotency_keys TO app_admin;
GRANT ALL ON SEQUENCE idempotency_keys_id_seq TO app_admin;

-- ===== FK entrantes a sessions (restaurar columnas nullable ON DELETE SET NULL) =====
-- Reverso de la limpieza del up (DROP CONSTRAINT + DROP COLUMN). El ADD COLUMN
-- con REFERENCES recrea la FK; Postgres regenera el nombre de constraint
-- (captured_prompts_session_id_fkey / verifications_session_id_fkey).
ALTER TABLE IF EXISTS captured_prompts
  ADD COLUMN IF NOT EXISTS session_id UUID REFERENCES sessions(id) ON DELETE SET NULL;
ALTER TABLE IF EXISTS verifications
  ADD COLUMN IF NOT EXISTS session_id UUID REFERENCES sessions(id) ON DELETE SET NULL;

COMMIT;
