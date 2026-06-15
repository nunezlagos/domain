DROP INDEX IF EXISTS skills_proposed_idx;
DROP INDEX IF EXISTS project_policies_proposed_idx;
DROP INDEX IF EXISTS skills_organization_idx;
DROP INDEX IF EXISTS project_policies_org_project_idx;
CREATE INDEX skills_organization_idx ON skills (organization_id) WHERE deleted_at IS NULL;
CREATE INDEX project_policies_org_project_idx
  ON project_policies (organization_id, project_id)
  WHERE deleted_at IS NULL AND is_active = TRUE;
ALTER TABLE skills DROP COLUMN IF EXISTS proposed;
ALTER TABLE project_policies DROP COLUMN IF EXISTS proposed;
