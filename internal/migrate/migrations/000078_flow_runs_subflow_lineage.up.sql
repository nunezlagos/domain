-- migration: flow_runs_subflow_lineage
-- author: nunezlagos
-- issue: issue-09.5
-- description: lineage de sub-flows en flow_runs (parent_run_id, ancestor_slugs, depth)
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE flow_runs
  ADD COLUMN IF NOT EXISTS parent_run_id UUID REFERENCES flow_runs(id) ON DELETE SET NULL;
ALTER TABLE flow_runs
  ADD COLUMN IF NOT EXISTS ancestor_slugs TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE flow_runs
  ADD COLUMN IF NOT EXISTS depth INT NOT NULL DEFAULT 0;

-- domain-lint-ignore-next: require-concurrent-index-creation
-- reason: columna recién agregada, sin filas con parent todavía
CREATE INDEX IF NOT EXISTS flow_runs_parent_run_idx
  ON flow_runs (parent_run_id) WHERE parent_run_id IS NOT NULL;
