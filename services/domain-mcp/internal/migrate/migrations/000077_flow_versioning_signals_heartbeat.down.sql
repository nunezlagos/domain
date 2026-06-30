

ALTER TABLE flow_versions
  DROP COLUMN IF EXISTS deprecated_at,
  DROP COLUMN IF EXISTS published_at,
  DROP COLUMN IF EXISTS is_default,
  DROP COLUMN IF EXISTS status;

ALTER TABLE flow_runs
  DROP COLUMN IF EXISTS flow_version_id;

DROP TABLE IF EXISTS flow_run_signals_pending;

ALTER TABLE flow_run_steps
  DROP COLUMN IF EXISTS heartbeat_threshold_seconds,
  DROP COLUMN IF EXISTS progress_message,
  DROP COLUMN IF EXISTS progress;
