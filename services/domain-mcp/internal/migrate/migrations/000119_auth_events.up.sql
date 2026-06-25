










CREATE TABLE IF NOT EXISTS auth_events (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
  organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,


  kind            VARCHAR(40) NOT NULL,

  email_attempted VARCHAR(255),
  success         BOOLEAN NOT NULL,


  reason          VARCHAR(80),
  ip              INET,
  user_agent      TEXT,

  session_id      UUID REFERENCES auth_sessions(id) ON DELETE SET NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);




CREATE INDEX IF NOT EXISTS auth_events_user_idx
  ON auth_events (user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS auth_events_email_ip_idx
  ON auth_events (email_attempted, ip, created_at DESC)
  WHERE success = FALSE;
CREATE INDEX IF NOT EXISTS auth_events_kind_idx
  ON auth_events (kind, created_at DESC);
CREATE INDEX IF NOT EXISTS auth_events_org_idx
  ON auth_events (organization_id, created_at DESC) WHERE organization_id IS NOT NULL;

GRANT SELECT, INSERT ON auth_events TO app_user;
GRANT ALL ON auth_events TO app_admin;
