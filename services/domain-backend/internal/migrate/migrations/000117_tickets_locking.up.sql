-- migration: tickets_locking
-- author: mnunez@saargo.com
-- issue: REQ-63 ticket locking + optimistic concurrency
-- description: soft lock cooperativo en tickets (señal "alguien está
--   trabajando aquí") + version field para optimistic concurrency en
--   updates concurrentes. El lock es informativo: si otro user intenta
--   modificar un ticket lockeado, el server responde 409 con info de
--   quién lo tiene. Self-expiring via locked_until.
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE project_tickets
  ADD COLUMN IF NOT EXISTS locked_by    UUID REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS version      INTEGER NOT NULL DEFAULT 1;

-- Index para "qué tickets tengo lockeados" o "qué está activo"
CREATE INDEX IF NOT EXISTS project_tickets_locked_idx
  ON project_tickets (organization_id, locked_by, locked_until)
  WHERE locked_by IS NOT NULL AND deleted_at IS NULL;
