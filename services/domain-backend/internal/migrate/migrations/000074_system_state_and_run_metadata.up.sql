-- migration: system_state_and_run_metadata
-- author: nunezlagos
-- issue: issue-08.12 orphan-runs-audit
-- description: tabla system_state para persistir state interno de crons (last_ack_at, etc.) + agent_runs.metadata JSONB para flag WithStandalone
-- breaking: false
-- estimated_duration: <1s (aditivo, sin lock largo)

BEGIN;

CREATE TABLE IF NOT EXISTS system_state (
  key VARCHAR(100) PRIMARY KEY,
  value JSONB NOT NULL DEFAULT '{}',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

GRANT SELECT, INSERT, UPDATE, DELETE ON system_state TO app_user;
DO $g$ BEGIN
  EXECUTE 'GRANT SELECT, INSERT, UPDATE, DELETE ON system_state TO app_admin';
EXCEPTION WHEN OTHERS THEN NULL;
END $g$;
DO $g$ BEGIN
  EXECUTE 'GRANT SELECT ON system_state TO app_readonly';
EXCEPTION WHEN OTHERS THEN NULL;
END $g$;

-- agent_runs.metadata: usado por WithStandalone() para marcar runs intencionales
-- sin flow_run_id ({"standalone":true, "reason":"debug|script|test"})
ALTER TABLE agent_runs ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMIT;
