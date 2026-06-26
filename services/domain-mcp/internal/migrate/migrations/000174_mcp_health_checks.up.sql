-- migration: 000174_mcp_health_checks
-- description: tabla de metrica de plataforma para el monitoreo de uptime/health
--   del propio server domain-mcp. domain-admin (Django) hace polling al /health
--   del MCP y registra una fila por chequeo (up/down/degraded + latencia).
--   Es metrica de PLATAFORMA (no por-org), por eso NO lleva organization_id ni
--   RLS (el RLS por org se deshabilito globalmente en la mig 000132).
-- breaking: no (tabla nueva, sin backfill).

CREATE TABLE IF NOT EXISTS mcp_health_checks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  status VARCHAR(10) NOT NULL,

  latency_ms INT,
  http_status INT,
  error TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT mcp_health_checks_status_check
    CHECK (status IN ('up', 'down', 'degraded'))
);

CREATE INDEX IF NOT EXISTS mcp_health_checks_checked_at_idx
  ON mcp_health_checks (checked_at DESC);

-- Grants: domain-admin escribe (poller) y lee (vista de uptime) con app_user.
-- app_admin mantiene control total (consistente con el resto de tablas).
GRANT SELECT, INSERT, UPDATE, DELETE ON mcp_health_checks TO app_user;
GRANT ALL ON mcp_health_checks TO app_admin;
