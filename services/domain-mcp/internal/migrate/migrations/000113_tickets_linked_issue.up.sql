-- migration: tickets_linked_issue
-- author: mnunez@saargo.com
-- issue: REQ-56 puente ticket↔issue (HU) — Opción A
-- description: agrega project_tickets.linked_issue_id como FK opcional a
--   la tabla issues (HUs del workflow SDD). Un ticket operativo puede
--   referenciar la HU formal que implementa.
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE project_tickets
  ADD COLUMN IF NOT EXISTS linked_issue_id UUID REFERENCES issues(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS project_tickets_linked_issue_idx
  ON project_tickets (linked_issue_id)
  WHERE linked_issue_id IS NOT NULL AND deleted_at IS NULL;
