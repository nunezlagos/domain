DROP INDEX IF EXISTS skills_org_project_idx;
DROP INDEX IF EXISTS skills_org_project_slug_uniq;
DROP INDEX IF EXISTS skills_org_slug_global_uniq;
ALTER TABLE skills DROP COLUMN IF EXISTS project_id;

ALTER TABLE skills ADD CONSTRAINT skills_organization_id_slug_key
  UNIQUE (organization_id, slug);
