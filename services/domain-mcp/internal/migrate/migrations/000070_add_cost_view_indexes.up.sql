
















CREATE INDEX IF NOT EXISTS agent_runs_org_day_idx
  ON agent_runs (organization_id, created_at DESC);


CREATE INDEX IF NOT EXISTS agent_runs_cost_view_idx
  ON agent_runs (status, organization_id, created_at DESC);
