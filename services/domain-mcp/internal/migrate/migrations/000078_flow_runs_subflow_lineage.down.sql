-- migration: flow_runs_subflow_lineage (down)
-- author: nunezlagos
-- issue: issue-09.5
-- description: revierte lineage de sub-flows
-- breaking: false
-- estimated_duration: <1s

DROP INDEX IF EXISTS flow_runs_parent_run_idx;
ALTER TABLE flow_runs DROP COLUMN IF EXISTS depth;
ALTER TABLE flow_runs DROP COLUMN IF EXISTS ancestor_slugs;
ALTER TABLE flow_runs DROP COLUMN IF EXISTS parent_run_id;
