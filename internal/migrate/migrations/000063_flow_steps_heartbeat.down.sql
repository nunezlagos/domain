DROP INDEX IF EXISTS flow_run_steps_heartbeat_idx;
ALTER TABLE flow_run_steps DROP COLUMN IF EXISTS last_heartbeat_at;
