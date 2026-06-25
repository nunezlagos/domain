






CREATE TABLE IF NOT EXISTS skill_executions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  skill_id UUID NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
  version_used INT,
  mode VARCHAR(10) NOT NULL DEFAULT 'sync'
    CHECK (mode IN ('sync','async')),
  status VARCHAR(20) NOT NULL DEFAULT 'completed'
    CHECK (status IN ('pending','running','completed','failed')),
  parameters JSONB NOT NULL DEFAULT '{}',
  output TEXT,
  error TEXT,
  execution_time_ms INT,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);



CREATE INDEX IF NOT EXISTS skill_executions_org_created_idx
  ON skill_executions (organization_id, created_at DESC);


CREATE INDEX IF NOT EXISTS skill_executions_pending_idx
  ON skill_executions (status) WHERE status IN ('pending','running');

GRANT SELECT, INSERT, UPDATE ON skill_executions TO app_user;
