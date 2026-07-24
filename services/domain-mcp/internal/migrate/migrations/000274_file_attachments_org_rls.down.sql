-- Reversa de 000274: quita RLS + organization_id de file_attachments.
DROP POLICY IF EXISTS file_attachments_org_isolation ON file_attachments;
ALTER TABLE file_attachments NO FORCE ROW LEVEL SECURITY;
ALTER TABLE file_attachments DISABLE ROW LEVEL SECURITY;
DROP INDEX IF EXISTS file_attachments_org_idx;
ALTER TABLE file_attachments DROP COLUMN IF EXISTS organization_id;
