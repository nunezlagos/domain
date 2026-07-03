-- Revierte REQ-56 issue-56.2.
DROP INDEX IF EXISTS idx_error_events_alive;
ALTER TABLE error_events
  DROP COLUMN IF EXISTS deletion_reason,
  DROP COLUMN IF EXISTS deleted_by,
  DROP COLUMN IF EXISTS deleted_at;

DROP TABLE IF EXISTS error_decision_log;
