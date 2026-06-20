-- migration: project_index_runs
-- author: mnunez@saargo.com
-- issue: REQ-62 project indexer estilo Cursor
-- description: tracking de cada "indexing run" del proyecto. El LLM
--   dispara domain_project_index_start al primer bootstrap (o re-index
--   manual), submitéa archivos del repo, server clasifica y persiste
--   en project_policies / knowledge_base / observations.
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE project_index_runs (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  started_by      UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  status          VARCHAR(20) NOT NULL DEFAULT 'running'
    CHECK (status IN ('running','completed','failed','partial')),
  git_head        VARCHAR(40),
  files_submitted INTEGER NOT NULL DEFAULT 0,
  -- summary: { policies_created, knowledge_created, skills_created,
  --           observations_created, tech_stack: {...}, ignored: [] }
  summary         JSONB NOT NULL DEFAULT '{}',
  started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at    TIMESTAMPTZ
);

CREATE INDEX project_index_runs_project_idx
  ON project_index_runs (organization_id, project_id, started_at DESC);
CREATE INDEX project_index_runs_status_idx
  ON project_index_runs (organization_id, status, started_at DESC)
  WHERE status IN ('running','partial');

ALTER TABLE project_index_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_index_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY project_index_runs_org_isolation ON project_index_runs
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON project_index_runs TO app_user;
GRANT ALL ON project_index_runs TO app_admin;
