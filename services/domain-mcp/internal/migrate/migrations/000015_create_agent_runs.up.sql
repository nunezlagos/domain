-- migration: create_agent_runs
-- author: mnunez@saargo.com
-- issue: HU-01.1 + HU-08.3
-- description: agent_runs con tokens, cost, parent_run para multi-agent (HU-08.6)
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE agent_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
  flow_run_id UUID REFERENCES flow_runs(id) ON DELETE CASCADE,
  parent_run_id UUID REFERENCES agent_runs(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  status VARCHAR(30) NOT NULL DEFAULT 'pending',
  inputs JSONB NOT NULL DEFAULT '{}',
  outputs JSONB,
  error TEXT,
  cancellation_reason VARCHAR(100),
  tokens_input BIGINT NOT NULL DEFAULT 0,
  tokens_output BIGINT NOT NULL DEFAULT 0,
  cost_usd NUMERIC(12,6) NOT NULL DEFAULT 0,
  iterations INT NOT NULL DEFAULT 0,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled'))
);

CREATE TRIGGER set_updated_at_agent_runs
  BEFORE UPDATE ON agent_runs
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX agent_runs_agent_idx ON agent_runs (agent_id, created_at DESC);
CREATE INDEX agent_runs_flow_run_idx ON agent_runs (flow_run_id) WHERE flow_run_id IS NOT NULL;
CREATE INDEX agent_runs_parent_idx ON agent_runs (parent_run_id) WHERE parent_run_id IS NOT NULL;
CREATE INDEX agent_runs_user_idx ON agent_runs (user_id) WHERE user_id IS NOT NULL;
