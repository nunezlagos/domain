-- migration: create_selfhosted_runners
-- author: nunezlagos
-- issue: HU-11.2
-- description: registry de runners selfhosted + queue de tasks
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE selfhosted_runners (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name VARCHAR(120) NOT NULL,
  api_key_hash VARCHAR(80) NOT NULL,
  labels TEXT[] NOT NULL DEFAULT '{}',
  last_heartbeat TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(organization_id, name)
);

CREATE INDEX selfhosted_runners_org_idx ON selfhosted_runners (organization_id);
CREATE INDEX selfhosted_runners_heartbeat_idx ON selfhosted_runners (last_heartbeat);

CREATE TABLE selfhosted_tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  kind VARCHAR(40) NOT NULL,
  required_labels TEXT[] NOT NULL DEFAULT '{}',
  payload JSONB NOT NULL,
  status VARCHAR(20) NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued','claimed','done','failed')),
  claimed_by UUID REFERENCES selfhosted_runners(id) ON DELETE SET NULL,
  claimed_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  result JSONB,
  error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX selfhosted_tasks_org_status_idx
  ON selfhosted_tasks (organization_id, status, created_at);
CREATE INDEX selfhosted_tasks_claimed_idx
  ON selfhosted_tasks (claimed_at) WHERE status = 'claimed';

GRANT SELECT, INSERT, UPDATE, DELETE ON selfhosted_runners TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON selfhosted_tasks TO app_user;
