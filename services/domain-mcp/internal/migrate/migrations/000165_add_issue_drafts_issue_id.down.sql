DROP INDEX IF EXISTS issue_drafts_issue_id_idx;
ALTER TABLE issue_drafts DROP COLUMN IF EXISTS issue_id;
