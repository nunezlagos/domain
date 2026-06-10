-- migration: flow_steps_heartbeat
-- author: nunezlagos
-- issue: HU-09.10
-- description: agrega last_heartbeat_at a flow_run_steps para watchdog
-- breaking: false
-- estimated_duration: <1s

ALTER TABLE flow_run_steps ADD COLUMN IF NOT EXISTS last_heartbeat_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS flow_run_steps_heartbeat_idx
  ON flow_run_steps (last_heartbeat_at)
  WHERE status = 'running';
