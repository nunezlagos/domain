










ALTER TABLE skills
  ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id) ON DELETE CASCADE;


DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'skills_organization_id_slug_key'
  ) THEN
    ALTER TABLE skills DROP CONSTRAINT skills_organization_id_slug_key;
  END IF;
END$$;




CREATE UNIQUE INDEX IF NOT EXISTS skills_org_slug_global_uniq
  ON skills (organization_id, slug)
  WHERE project_id IS NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS skills_org_project_slug_uniq
  ON skills (organization_id, project_id, slug)
  WHERE project_id IS NOT NULL AND deleted_at IS NULL;


CREATE INDEX IF NOT EXISTS skills_org_project_idx
  ON skills (organization_id, project_id)
  WHERE deleted_at IS NULL;
