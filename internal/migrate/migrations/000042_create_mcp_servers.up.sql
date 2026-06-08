-- migration: create_mcp_servers
-- author: nunezlagos
-- issue: HU-12.4
-- description: registro de MCP servers externos consumidos por Domain + cache de sus tools
-- breaking: false
-- estimated_duration: <1s

-- Servers MCP externos registrados por una org. Domain spawnea/conecta y
-- mantiene sus tools como skills derivados.
CREATE TABLE IF NOT EXISTS mcp_servers (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name            VARCHAR(80) NOT NULL,
  transport       VARCHAR(20) NOT NULL DEFAULT 'stdio'
    CHECK (transport IN ('stdio','http','sse')),
  command         TEXT,
  args            TEXT[] NOT NULL DEFAULT '{}',
  env_cipher      BYTEA,
  url             TEXT,
  enabled         BOOLEAN NOT NULL DEFAULT TRUE,
  status          VARCHAR(20) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','connected','disconnected','failed')),
  last_connected_at TIMESTAMPTZ,
  last_error        TEXT,
  retry_count       INTEGER NOT NULL DEFAULT 0,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT mcp_servers_org_name_unique UNIQUE (organization_id, name),
  CONSTRAINT mcp_servers_transport_check CHECK (
    (transport = 'stdio' AND command IS NOT NULL) OR
    (transport IN ('http','sse') AND url IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS mcp_servers_org_enabled_idx
  ON mcp_servers(organization_id) WHERE enabled = TRUE;
CREATE INDEX IF NOT EXISTS mcp_servers_status_idx
  ON mcp_servers(status) WHERE enabled = TRUE;

CREATE TRIGGER set_updated_at_mcp_servers
  BEFORE UPDATE ON mcp_servers
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Cache de tools descubiertos por servidor (resultado de tools/list).
-- Cada tool se materializa como skill ejecutable.
CREATE TABLE IF NOT EXISTS mcp_server_tools (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  mcp_server_id   UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  tool_name       VARCHAR(120) NOT NULL,
  description     TEXT NOT NULL DEFAULT '',
  input_schema    JSONB NOT NULL DEFAULT '{}',
  enabled         BOOLEAN NOT NULL DEFAULT TRUE,
  discovered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT mcp_server_tools_unique UNIQUE (mcp_server_id, tool_name)
);

CREATE INDEX IF NOT EXISTS mcp_server_tools_org_idx
  ON mcp_server_tools(organization_id) WHERE enabled = TRUE;

CREATE TRIGGER set_updated_at_mcp_server_tools
  BEFORE UPDATE ON mcp_server_tools
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

GRANT SELECT, INSERT, UPDATE, DELETE ON mcp_servers TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON mcp_server_tools TO app_user;
GRANT SELECT ON mcp_servers TO app_readonly;
GRANT SELECT ON mcp_server_tools TO app_readonly;
