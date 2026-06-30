CREATE TABLE project_merges (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  target_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  report JSONB NOT NULL DEFAULT '[]',
  actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
  merged_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (source_id != target_id)
);

CREATE INDEX project_merges_target_idx ON project_merges (target_id, created_at DESC);
CREATE INDEX project_merges_source_idx ON project_merges (source_id, created_at DESC);
