






CREATE TABLE flow_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  flow_id UUID NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
  triggered_by UUID REFERENCES users(id) ON DELETE SET NULL,
  trigger_type VARCHAR(30) NOT NULL DEFAULT 'manual',
  status VARCHAR(30) NOT NULL DEFAULT 'pending',
  inputs JSONB NOT NULL DEFAULT '{}',
  outputs JSONB,
  error TEXT,
  cursor JSONB NOT NULL DEFAULT '{}',
  worker_id VARCHAR(64),
  last_heartbeat_at TIMESTAMPTZ,
  recovery_count INT NOT NULL DEFAULT 0,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (status IN ('pending', 'running', 'completed', 'failed', 'paused', 'cancelled', 'paused_awaiting_signal', 'paused_awaiting_human'))
);

CREATE TRIGGER set_updated_at_flow_runs
  BEFORE UPDATE ON flow_runs
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX flow_runs_status_pending_idx ON flow_runs (status, worker_id) WHERE status IN ('pending', 'running');
CREATE INDEX flow_runs_flow_idx ON flow_runs (flow_id, created_at DESC);
CREATE INDEX flow_runs_recovery_idx ON flow_runs (status, last_heartbeat_at) WHERE status = 'running';
