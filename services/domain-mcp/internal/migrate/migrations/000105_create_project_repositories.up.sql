













CREATE TABLE project_repositories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,


  name VARCHAR(50) NOT NULL,
  url VARCHAR(500) NOT NULL,


  branch_default VARCHAR(100),


  kind VARCHAR(40),

  is_default BOOLEAN NOT NULL DEFAULT false,


  workflow VARCHAR(40),
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  UNIQUE (organization_id, project_id, name),
  CHECK (name <> ''),
  CHECK (url <> '')
);


CREATE UNIQUE INDEX project_repositories_one_default_idx
  ON project_repositories (organization_id, project_id)
  WHERE is_default = true AND deleted_at IS NULL;

CREATE INDEX project_repositories_org_project_idx
  ON project_repositories (organization_id, project_id)
  WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at_project_repositories
  BEFORE UPDATE ON project_repositories
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();


ALTER TABLE project_repositories ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_repositories FORCE ROW LEVEL SECURITY;
CREATE POLICY project_repositories_org_isolation ON project_repositories
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON project_repositories TO app_user;
GRANT ALL ON project_repositories TO app_admin;
