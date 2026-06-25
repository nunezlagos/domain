








ALTER TABLE project_tickets
  ADD COLUMN IF NOT EXISTS linked_issue_id UUID REFERENCES issues(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS project_tickets_linked_issue_idx
  ON project_tickets (linked_issue_id)
  WHERE linked_issue_id IS NOT NULL AND deleted_at IS NULL;
