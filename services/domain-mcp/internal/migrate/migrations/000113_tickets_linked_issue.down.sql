DROP INDEX IF EXISTS project_tickets_linked_issue_idx;
ALTER TABLE project_tickets DROP COLUMN IF EXISTS linked_issue_id;
