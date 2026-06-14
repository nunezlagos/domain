-- migration: create_project_policies
-- author: mnunez@saargo.com
-- issue: REQ-43 policies por proyecto (Ola B)
-- description: policies scoped a (org, project). Resolver jerárquico:
--   project_policies → platform_policies (fallback). Mismo schema base
--   que platform_policies pero con scope explícito + RLS.
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE project_policies (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  slug            VARCHAR(80) NOT NULL,
  name            VARCHAR(160) NOT NULL,
  kind            VARCHAR(40) NOT NULL
    CHECK (kind IN (
      'convention','security_rule','architecture','sdd_workflow',
      'observability','migration_rule','linter_config','agent_protocol',
      'git_workflow','tech_stack','test_strategy'
    )),
  body_md         TEXT NOT NULL,
  body_structured JSONB NOT NULL DEFAULT '{}',
  version         INTEGER NOT NULL DEFAULT 1,
  is_active       BOOLEAN NOT NULL DEFAULT TRUE,
  -- override_platform: si true, esta policy OVERRIDE la del platform con
  -- el mismo slug. Si false (default), AMPLÍA (LLM ve ambas concatenadas).
  override_platform BOOLEAN NOT NULL DEFAULT FALSE,
  source           VARCHAR(40) NOT NULL DEFAULT 'manual'
    CHECK (source IN ('manual','llm_generated','seed_imported','dashboard')),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at      TIMESTAMPTZ,
  -- 1 policy activa por (org, project, slug). Active=false permite tener
  -- versiones inactivas con el mismo slug en historial.
  CONSTRAINT project_policies_slug_active_unique
    UNIQUE (organization_id, project_id, slug, is_active)
);

CREATE INDEX project_policies_org_project_idx
  ON project_policies (organization_id, project_id)
  WHERE deleted_at IS NULL AND is_active = TRUE;
CREATE INDEX project_policies_kind_idx
  ON project_policies (organization_id, project_id, kind)
  WHERE deleted_at IS NULL AND is_active = TRUE;
CREATE INDEX project_policies_slug_idx
  ON project_policies (organization_id, project_id, slug)
  WHERE deleted_at IS NULL;

CREATE TRIGGER set_updated_at_project_policies
  BEFORE UPDATE ON project_policies
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Versiones históricas
CREATE TABLE project_policy_versions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  policy_id       UUID NOT NULL REFERENCES project_policies(id) ON DELETE CASCADE,
  version         INTEGER NOT NULL,
  body_md         TEXT NOT NULL,
  body_structured JSONB NOT NULL DEFAULT '{}',
  changed_by      UUID,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (policy_id, version)
);

-- RLS (defense-in-depth multi-tenant)
ALTER TABLE project_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_policies FORCE ROW LEVEL SECURITY;
CREATE POLICY project_policies_org_isolation ON project_policies
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

ALTER TABLE project_policy_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_policy_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY project_policy_versions_via_parent ON project_policy_versions
  FOR ALL TO PUBLIC
  USING (
    EXISTS (
      SELECT 1 FROM project_policies p
      WHERE p.id = policy_id AND p.organization_id = current_org_id()
    )
  )
  WITH CHECK (
    EXISTS (
      SELECT 1 FROM project_policies p
      WHERE p.id = policy_id AND p.organization_id = current_org_id()
    )
  );

GRANT SELECT, INSERT, UPDATE, DELETE ON project_policies TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON project_policy_versions TO app_user;
GRANT ALL ON project_policies, project_policy_versions TO app_admin;
