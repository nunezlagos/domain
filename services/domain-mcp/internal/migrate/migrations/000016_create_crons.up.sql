






CREATE TABLE crons (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  slug VARCHAR(100) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  cron_expression VARCHAR(100) NOT NULL,
  timezone VARCHAR(50) NOT NULL DEFAULT 'UTC',
  target_type VARCHAR(20) NOT NULL,
  target_id UUID NOT NULL,
  inputs JSONB NOT NULL DEFAULT '{}',
  enabled BOOLEAN NOT NULL DEFAULT true,
  last_run_at TIMESTAMPTZ,
  next_run_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (organization_id, slug),
  CHECK (target_type IN ('flow', 'agent', 'skill'))
);

CREATE TRIGGER set_updated_at_crons
  BEFORE UPDATE ON crons
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX crons_next_run_idx ON crons (next_run_at) WHERE enabled = true AND deleted_at IS NULL;
