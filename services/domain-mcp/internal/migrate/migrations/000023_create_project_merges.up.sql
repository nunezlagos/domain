






CREATE TABLE project_merges (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  source_project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  target_project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  merge_log JSONB NOT NULL DEFAULT '[]',
  status VARCHAR(20) NOT NULL DEFAULT 'pending',
  error TEXT,
  initiated_by UUID REFERENCES users(id) ON DELETE SET NULL,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (status IN ('pending', 'running', 'completed', 'failed', 'rolled_back')),
  CHECK (source_project_id != target_project_id)
);

CREATE INDEX project_merges_target_idx ON project_merges (target_project_id, created_at DESC);
CREATE INDEX project_merges_source_idx ON project_merges (source_project_id, created_at DESC);
