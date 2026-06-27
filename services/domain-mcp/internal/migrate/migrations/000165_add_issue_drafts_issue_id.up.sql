-- migration: 000165_add_issue_drafts_issue_id
-- author: NunezLagos
-- issue: legacy
-- description: columna issue_id (FK a issues) en issue_drafts + indice
-- breaking: no
-- estimated_duration: unknown

ALTER TABLE issue_drafts ADD COLUMN issue_id UUID REFERENCES issues(id) ON DELETE SET NULL;





CREATE INDEX IF NOT EXISTS issue_drafts_issue_id_idx ON issue_drafts (issue_id);
