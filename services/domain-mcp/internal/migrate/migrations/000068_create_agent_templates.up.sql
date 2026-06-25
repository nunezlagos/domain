






CREATE TABLE agent_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  slug VARCHAR(80) NOT NULL,
  name VARCHAR(120) NOT NULL,
  system_prompt TEXT NOT NULL,
  personality TEXT,
  capabilities TEXT[] NOT NULL DEFAULT '{}',  -- slugs de skills disponibles
  model VARCHAR(80) NOT NULL DEFAULT 'claude-haiku-4-5',
  temperature NUMERIC(3,2) NOT NULL DEFAULT 0.7,
  max_tokens INT NOT NULL DEFAULT 4096,
  handoff_policy VARCHAR(40) NOT NULL DEFAULT 'allow'
    CHECK (handoff_policy IN ('allow','forbid','require_supervisor_approval')),
  metadata JSONB NOT NULL DEFAULT '{}',
  created_by UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(organization_id, slug)
);

CREATE INDEX agent_templates_org_idx ON agent_templates (organization_id);

CREATE TRIGGER set_updated_at_agent_templates
  BEFORE UPDATE ON agent_templates
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON agent_templates TO app_user;
