






ALTER TABLE flow_runs
  ADD COLUMN IF NOT EXISTS parent_run_id UUID REFERENCES flow_runs(id) ON DELETE SET NULL;
ALTER TABLE flow_runs
  ADD COLUMN IF NOT EXISTS ancestor_slugs TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE flow_runs
  ADD COLUMN IF NOT EXISTS depth INT NOT NULL DEFAULT 0;



CREATE INDEX IF NOT EXISTS flow_runs_parent_run_idx
  ON flow_runs (parent_run_id) WHERE parent_run_id IS NOT NULL;
