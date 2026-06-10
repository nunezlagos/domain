-- rollback add_cost_view_indexes
DROP INDEX IF EXISTS agent_runs_org_day_idx;
DROP INDEX IF EXISTS agent_runs_cost_view_idx;
