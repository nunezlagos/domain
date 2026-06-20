-- migration: create_project_repositories
-- author: mnunez@saargo.com
-- issue: REQ-42 multi-remotos por proyecto
-- description: un proyecto puede tener N remotos (github + gitlab espejo,
--   subtree de monorepo, mirror interno, etc.). El user/LLM puede tener
--   ambigüedad al decidir "dónde push": esta tabla guarda los remotos
--   conocidos por proyecto y cuál es default.
-- breaking: false
-- estimated_duration: <1s
--
-- projects.repository_url queda como compat (1 remoto principal); este
-- tabla extiende. Si projects.repository_url no está vacío, el seeder o
-- el endpoint /project_repo_sync puede backfillearlo como default.

CREATE TABLE project_repositories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  -- name: alias del remoto (origin, upstream, gitlab, mirror, ...).
  -- Coincide con `git remote -v`.
  name VARCHAR(50) NOT NULL,
  url VARCHAR(500) NOT NULL,
  -- branch_default: la rama "principal" en este remoto (main, master,
  -- develop, services, ...). Si vacío, el cliente debe consultar.
  branch_default VARCHAR(100),
  -- kind: github | gitlab | bitbucket | gitea | other. Útil para que el
  -- LLM elija API según provider (gh/glab/etc).
  kind VARCHAR(40),
  -- is_default: cuál usar si el LLM no pregunta. Único por (org, project).
  is_default BOOLEAN NOT NULL DEFAULT false,
  -- workflow: merge | pr | mr | trunk_based — convención de cómo
  -- llevar cambios a este remoto.
  workflow VARCHAR(40),
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,

  UNIQUE (organization_id, project_id, name),
  CHECK (name <> ''),
  CHECK (url <> '')
);

-- Solo 1 default por proyecto activo.
CREATE UNIQUE INDEX project_repositories_one_default_idx
  ON project_repositories (organization_id, project_id)
  WHERE is_default = true AND deleted_at IS NULL;

CREATE INDEX project_repositories_org_project_idx
  ON project_repositories (organization_id, project_id)
  WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at_project_repositories
  BEFORE UPDATE ON project_repositories
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- RLS (defense-in-depth)
ALTER TABLE project_repositories ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_repositories FORCE ROW LEVEL SECURITY;
CREATE POLICY project_repositories_org_isolation ON project_repositories
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON project_repositories TO app_user;
GRANT ALL ON project_repositories TO app_admin;
