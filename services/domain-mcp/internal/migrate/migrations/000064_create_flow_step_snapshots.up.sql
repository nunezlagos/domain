






CREATE TABLE flow_run_step_snapshots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  step_id UUID NOT NULL UNIQUE REFERENCES flow_run_steps(id) ON DELETE CASCADE,
  run_id UUID NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
  step_key VARCHAR(120) NOT NULL,
  input JSONB NOT NULL,
  output JSONB,
  error TEXT,
  duration_ms BIGINT NOT NULL DEFAULT 0,
  captured_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX flow_step_snapshots_run_idx ON flow_run_step_snapshots (run_id, captured_at);

GRANT SELECT, INSERT, UPDATE ON flow_run_step_snapshots TO app_user;
