-- REQ-56 issue-56.2: auditoría de las decisiones sobre errores.
-- Hoy domain_known_error_set (clasificar benigno/recoverable) y domain_error_reset
-- (borrar error_events) no dejan rastro de quién/cuándo/por qué. Y error_reset hace
-- DELETE duro sin posibilidad de revertir. Esta migración agrega:
--   1) error_decision_log: audit trail append-only de ambas decisiones.
--   2) columnas de soft-delete en error_events para que reset sea reversible.

-- 1) Audit trail de decisiones sobre errores (append-only).
CREATE TABLE IF NOT EXISTS error_decision_log (
  id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  fingerprint  bytea NOT NULL,
  action       text  NOT NULL
    CHECK (action IN ('known_error_set', 'error_reset')),
  actor_id     uuid  NULL,          -- Principal.UserID; NULL si no hay sesión autenticada
  reason       text  NULL,          -- razón que dio el operador (por qué es benigno / por qué se resetea)
  detail       jsonb NULL,          -- snapshot opcional (name, auto_heal_action, dedup_count previo, etc.)
  occurred_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_error_decision_log_fp
  ON error_decision_log (fingerprint, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_error_decision_log_actor
  ON error_decision_log (actor_id, occurred_at DESC);

-- Append-only: prohibir UPDATE/DELETE a nivel de rol de aplicación (mismo criterio
-- que audit_log en 000047). Los superusuarios/migraciones no se ven afectados.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'domain_app') THEN
    REVOKE UPDATE, DELETE ON error_decision_log FROM domain_app;
  END IF;
END $$;

-- 2) Soft-delete de error_events: reset deja de ser DELETE duro.
ALTER TABLE error_events
  ADD COLUMN IF NOT EXISTS deleted_at      timestamptz NULL,
  ADD COLUMN IF NOT EXISTS deleted_by      uuid        NULL,
  ADD COLUMN IF NOT EXISTS deletion_reason text        NULL;

-- Las lecturas "vivas" deben excluir los soft-deleted. Índice parcial para eso.
CREATE INDEX IF NOT EXISTS idx_error_events_alive
  ON error_events (fingerprint) WHERE deleted_at IS NULL;
