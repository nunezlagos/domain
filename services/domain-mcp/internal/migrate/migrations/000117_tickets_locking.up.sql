










ALTER TABLE project_tickets
  ADD COLUMN IF NOT EXISTS locked_by    UUID REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS version      INTEGER NOT NULL DEFAULT 1;


CREATE INDEX IF NOT EXISTS project_tickets_locked_idx
  ON project_tickets (organization_id, locked_by, locked_until)
  WHERE locked_by IS NOT NULL AND deleted_at IS NULL;
