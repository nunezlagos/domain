











ALTER TABLE issue_drafts ADD COLUMN issue_id UUID REFERENCES issues(id) ON DELETE SET NULL;





CREATE INDEX IF NOT EXISTS issue_drafts_issue_id_idx ON issue_drafts (issue_id);
