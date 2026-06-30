











ALTER TABLE project_policies
  ADD COLUMN IF NOT EXISTS proposed BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE skills
  ADD COLUMN IF NOT EXISTS proposed BOOLEAN NOT NULL DEFAULT false;



DROP INDEX IF EXISTS project_policies_org_project_idx;
CREATE INDEX project_policies_org_project_idx
  ON project_policies (organization_id, project_id)
  WHERE deleted_at IS NULL AND is_active = TRUE AND proposed = false;

DROP INDEX IF EXISTS skills_organization_idx;
CREATE INDEX skills_organization_idx
  ON skills (organization_id)
  WHERE deleted_at IS NULL AND proposed = false;


CREATE INDEX IF NOT EXISTS project_policies_proposed_idx
  ON project_policies (organization_id, project_id, created_at DESC)
  WHERE proposed = true AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS skills_proposed_idx
  ON skills (organization_id, created_at DESC)
  WHERE proposed = true AND deleted_at IS NULL;
