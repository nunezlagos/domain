






CREATE TABLE agents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  slug VARCHAR(100) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  provider VARCHAR(50) NOT NULL,
  model VARCHAR(100) NOT NULL,
  system_prompt TEXT,
  skills_slugs TEXT[] NOT NULL DEFAULT '{}',
  max_iterations INT NOT NULL DEFAULT 20,
  token_budget BIGINT,
  temperature NUMERIC(3,2),
  seed_managed BOOLEAN NOT NULL DEFAULT false,
  seed_version INT,
  is_user_modified BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (organization_id, slug)
);

CREATE TRIGGER set_updated_at_agents
  BEFORE UPDATE ON agents
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX agents_organization_idx ON agents (organization_id) WHERE deleted_at IS NULL;
