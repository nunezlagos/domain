CREATE TABLE IF NOT EXISTS workflows (
  id uuid PRIMARY KEY,
  name text NULL,
  status text NOT NULL CHECK (status IN ('running','completed','failed','abandoned')),
  started_at timestamptz NOT NULL DEFAULT now(),
  ended_at timestamptz NULL,
  total_tool_calls int NOT NULL DEFAULT 0,
  total_errors int NOT NULL DEFAULT 0,
  total_duration_ms bigint NOT NULL DEFAULT 0,
  actor_id uuid NULL,
  api_key_id uuid NULL,
  project_id uuid NULL,
  last_activity_at timestamptz NOT NULL DEFAULT now(),
  metadata jsonb
);

CREATE INDEX IF NOT EXISTS idx_workflows_status_activity
  ON workflows (status, last_activity_at DESC);
CREATE INDEX IF NOT EXISTS idx_workflows_actor_started
  ON workflows (actor_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_workflows_project_started
  ON workflows (project_id, started_at DESC);

CREATE TABLE IF NOT EXISTS workflow_steps (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  workflow_id uuid NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
  step_type text NOT NULL CHECK (step_type IN ('tool','fn','sql','http')),
  ref_name text NOT NULL,
  status text NOT NULL,
  duration_ms int NOT NULL,
  error_code text NULL,
  error_message text NULL,
  args_hash bytea NULL,
  started_at timestamptz NOT NULL DEFAULT now(),
  ended_at timestamptz NULL
);

CREATE INDEX IF NOT EXISTS idx_workflow_steps_workflow
  ON workflow_steps (workflow_id, started_at);
CREATE INDEX IF NOT EXISTS idx_workflow_steps_ref
  ON workflow_steps (ref_name, started_at DESC);
