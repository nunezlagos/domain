-- migration: create_cost_logs
-- author: mnunez@saargo.com
-- issue: HU-01.1 + HU-15.1 + RFC 0004
-- description: cost_logs source of truth de facturación (Prometheus solo SRE)
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE cost_logs (
  id BIGSERIAL PRIMARY KEY,
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  flow_run_id UUID REFERENCES flow_runs(id) ON DELETE SET NULL,
  agent_run_id UUID REFERENCES agent_runs(id) ON DELETE SET NULL,
  provider VARCHAR(50) NOT NULL,
  model VARCHAR(100) NOT NULL,
  operation VARCHAR(30) NOT NULL,
  tokens_input BIGINT NOT NULL DEFAULT 0,
  tokens_output BIGINT NOT NULL DEFAULT 0,
  tokens_cached BIGINT NOT NULL DEFAULT 0,
  cost_usd NUMERIC(12,6) NOT NULL,
  latency_ms INT,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (operation IN ('completion', 'embedding', 'image', 'audio', 'tool_call'))
);

CREATE INDEX cost_logs_org_occurred_idx ON cost_logs (organization_id, occurred_at DESC);
CREATE INDEX cost_logs_user_idx ON cost_logs (user_id, occurred_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX cost_logs_flow_run_idx ON cost_logs (flow_run_id) WHERE flow_run_id IS NOT NULL;
CREATE INDEX cost_logs_agent_run_idx ON cost_logs (agent_run_id) WHERE agent_run_id IS NOT NULL;
CREATE INDEX cost_logs_provider_model_idx ON cost_logs (provider, model, occurred_at DESC);
