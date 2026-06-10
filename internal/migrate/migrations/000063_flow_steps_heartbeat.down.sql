DROP INDEX IF EXISTS flow_run_steps_heartbeat_idx;
DROP INDEX IF EXISTS flow_run_steps_run_idx;
DROP TRIGGER IF EXISTS set_updated_at_flow_run_steps ON flow_run_steps;
DROP TABLE IF EXISTS flow_run_steps CASCADE;
