






CREATE TABLE flow_signals (
  id BIGSERIAL PRIMARY KEY,
  flow_run_id UUID NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
  step_key VARCHAR(120),
  name VARCHAR(60) NOT NULL,
  payload JSONB,
  delivered_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX flow_signals_pending_idx ON flow_signals (flow_run_id, name)
  WHERE delivered_at IS NULL;
CREATE INDEX flow_signals_created_idx ON flow_signals (created_at);

GRANT SELECT, INSERT, UPDATE ON flow_signals TO app_user;
GRANT USAGE, SELECT ON SEQUENCE flow_signals_id_seq TO app_user;
