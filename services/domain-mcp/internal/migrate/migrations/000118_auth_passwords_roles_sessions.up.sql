









ALTER TABLE users
  ADD COLUMN IF NOT EXISTS password_hash BYTEA,
  ADD COLUMN IF NOT EXISTS password_set_at TIMESTAMPTZ;




CREATE TABLE IF NOT EXISTS roles (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug        VARCHAR(40) UNIQUE NOT NULL,
  name        VARCHAR(120) NOT NULL,
  description TEXT,


  permissions TEXT[] NOT NULL DEFAULT '{}',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO roles (slug, name, description, permissions) VALUES
  ('admin', 'Administrador',
   'Acceso total: orgs, users, billing, audit, dashboard completo.',
   ARRAY['admin:all']),
  ('developer', 'Desarrollador',
   'Tickets, projects, observations, MCP tools, knowledge.',
   ARRAY['tickets:write','projects:read','observations:write','knowledge:write','mcp:invoke']),
  ('pm', 'Project Manager',
   'Tickets, projects, reportes, asignaciones.',
   ARRAY['tickets:write','projects:write','reports:read','tickets:reassign']),
  ('qa', 'QA',
   'Tickets + observations + verifications + status history.',
   ARRAY['tickets:write','observations:write','verifications:write']),
  ('viewer', 'Solo lectura',
   'Tickets/projects en read-only.',
   ARRAY['tickets:read','projects:read'])
ON CONFLICT (slug) DO NOTHING;



CREATE TABLE IF NOT EXISTS user_roles (
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  granted_by UUID REFERENCES users(id) ON DELETE SET NULL,
  PRIMARY KEY (user_id, role_id)
);

CREATE INDEX IF NOT EXISTS user_roles_user_idx ON user_roles (user_id);




CREATE TABLE IF NOT EXISTS auth_sessions (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  active_role_id  UUID NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
  token_hash      BYTEA NOT NULL UNIQUE,
  user_agent      TEXT,
  ip              INET,
  expires_at      TIMESTAMPTZ NOT NULL,
  last_used_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS auth_sessions_token_idx ON auth_sessions (token_hash)
  WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS auth_sessions_user_idx ON auth_sessions (user_id, expires_at DESC)
  WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS auth_sessions_cleanup_idx ON auth_sessions (expires_at)
  WHERE revoked_at IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON roles TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON user_roles TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON auth_sessions TO app_user;
GRANT ALL ON roles TO app_admin;
GRANT ALL ON user_roles TO app_admin;
GRANT ALL ON auth_sessions TO app_admin;
