








ALTER TABLE flow_versions
  ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft', 'published', 'deprecated'));
ALTER TABLE flow_versions
  ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE flow_versions
  ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ;
ALTER TABLE flow_versions
  ADD COLUMN IF NOT EXISTS deprecated_at TIMESTAMPTZ;


CREATE UNIQUE INDEX IF NOT EXISTS flow_versions_default_idx
  ON flow_versions (flow_id) WHERE is_default = true;


ALTER TABLE flow_runs
  ADD COLUMN IF NOT EXISTS flow_version_id UUID REFERENCES flow_versions(id);


CREATE TABLE IF NOT EXISTS flow_run_signals_pending (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  flow_run_id UUID NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
  step_id VARCHAR(100) NOT NULL,
  signal_name VARCHAR(100) NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (flow_run_id, step_id)
);
CREATE INDEX IF NOT EXISTS flow_run_signals_pending_name_idx
  ON flow_run_signals_pending (signal_name);


ALTER TABLE flow_run_steps
  ADD COLUMN IF NOT EXISTS progress DOUBLE PRECISION;
ALTER TABLE flow_run_steps
  ADD COLUMN IF NOT EXISTS progress_message TEXT;
ALTER TABLE flow_run_steps
  ADD COLUMN IF NOT EXISTS heartbeat_threshold_seconds INT DEFAULT 120;

GRANT SELECT, INSERT, UPDATE, DELETE ON flow_run_signals_pending TO app_user;
