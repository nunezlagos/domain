-- migration: create_flow_run_steps + heartbeat
-- author: nunezlagos
-- issue: HU-09.10 (heartbeat) + HU-09.3 (step state machine)
-- description: crea tabla flow_run_steps si no existe + last_heartbeat_at
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE IF NOT EXISTS flow_run_steps (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  flow_run_id UUID NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
  step_key VARCHAR(120) NOT NULL,
  status VARCHAR(30) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','running','completed','failed','skipped','blocked','cancelled')),
  inputs JSONB NOT NULL DEFAULT '{}',
  outputs JSONB,
  error TEXT,
  attempt INT NOT NULL DEFAULT 1,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  last_heartbeat_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE flow_run_steps ADD COLUMN IF NOT EXISTS last_heartbeat_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS flow_run_steps_run_idx
  ON flow_run_steps (flow_run_id, created_at);
CREATE INDEX IF NOT EXISTS flow_run_steps_heartbeat_idx
  ON flow_run_steps (last_heartbeat_at)
  WHERE status = 'running';

DO $trigger$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger WHERE tgname = 'set_updated_at_flow_run_steps'
  ) THEN
    CREATE TRIGGER set_updated_at_flow_run_steps
      BEFORE UPDATE ON flow_run_steps
      FOR EACH ROW EXECUTE FUNCTION set_updated_at();
  END IF;
END$trigger$;

GRANT SELECT, INSERT, UPDATE, DELETE ON flow_run_steps TO app_user;
