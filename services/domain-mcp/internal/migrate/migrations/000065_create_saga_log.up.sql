






CREATE TABLE saga_compensation_log (
  id BIGSERIAL PRIMARY KEY,
  run_id UUID NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
  original_step VARCHAR(120) NOT NULL,
  compensate_ran VARCHAR(120) NOT NULL,
  success BOOLEAN NOT NULL,
  error TEXT,
  payload JSONB,
  executed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX saga_log_run_idx ON saga_compensation_log (run_id, executed_at);

GRANT SELECT, INSERT ON saga_compensation_log TO app_user;
GRANT USAGE, SELECT ON SEQUENCE saga_compensation_log_id_seq TO app_user;
