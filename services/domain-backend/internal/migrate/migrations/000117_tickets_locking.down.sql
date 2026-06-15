DROP INDEX IF EXISTS project_tickets_locked_idx;
ALTER TABLE project_tickets
  DROP COLUMN IF EXISTS version,
  DROP COLUMN IF EXISTS locked_until,
  DROP COLUMN IF EXISTS locked_by;
