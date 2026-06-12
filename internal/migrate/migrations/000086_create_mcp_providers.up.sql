-- migration: create_mcp_providers
-- author: nunezlagos
-- issue: F1 (catálogo de MCPs instalables)
-- description: catálogo de MCP servers que domain puede autoinstalar
--              en clientes IA (opencode, claude-code, claude-desktop).
--              Diferente de mcp_servers (HU-12.4) que es para MCPs EXTERNOS
--              que domain CONSUME.
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE mcp_providers (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name            VARCHAR(80) NOT NULL,
  description     TEXT NOT NULL,
  command         TEXT NOT NULL,
  default_args    TEXT[] NOT NULL DEFAULT '{}',
  env_template    JSONB NOT NULL DEFAULT '{}',
  required_env    TEXT[] NOT NULL DEFAULT '{}',
  tags            TEXT[] NOT NULL DEFAULT '{}',
  is_built_in     BOOLEAN NOT NULL DEFAULT TRUE,
  is_public       BOOLEAN NOT NULL DEFAULT TRUE,
  organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT mcp_providers_name_unique UNIQUE (name)
);

CREATE INDEX mcp_providers_built_in_public_idx
  ON mcp_providers (is_built_in, is_public) WHERE organization_id IS NULL;
CREATE INDEX mcp_providers_org_idx
  ON mcp_providers (organization_id) WHERE organization_id IS NOT NULL;

CREATE TRIGGER set_updated_at_mcp_providers
  BEFORE UPDATE ON mcp_providers
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON mcp_providers TO app_user;
GRANT SELECT ON mcp_providers TO app_readonly;
