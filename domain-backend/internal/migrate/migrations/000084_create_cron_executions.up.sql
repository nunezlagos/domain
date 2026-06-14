-- migration: create_cron_executions
-- author: nunezlagos
-- issue: issue-10.1
-- description: historial de ejecuciones de crons (running/completed/failed/skipped_overlap)
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE IF NOT EXISTS cron_executions (
  id BIGSERIAL PRIMARY KEY,
  cron_id UUID NOT NULL REFERENCES crons(id) ON DELETE CASCADE,
  status VARCHAR(20) NOT NULL DEFAULT 'running'
    CHECK (status IN ('running','completed','failed','skipped_overlap')),
  target_type VARCHAR(20) NOT NULL,
  error TEXT,
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ,
  duration_ms INT
);

-- domain-lint-ignore-next: require-concurrent-index-creation
-- reason: tabla nueva sin tráfico
CREATE INDEX IF NOT EXISTS cron_executions_cron_started_idx
  ON cron_executions (cron_id, started_at DESC);
-- domain-lint-ignore-next: require-concurrent-index-creation
-- reason: tabla nueva sin tráfico
CREATE INDEX IF NOT EXISTS cron_executions_running_idx
  ON cron_executions (cron_id) WHERE status = 'running';

GRANT SELECT, INSERT, UPDATE ON cron_executions TO app_user;
GRANT USAGE ON SEQUENCE cron_executions_id_seq TO app_user;
GRANT ALL ON cron_executions TO app_admin;
GRANT ALL ON SEQUENCE cron_executions_id_seq TO app_admin;
