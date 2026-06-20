-- migration: auth_events
-- author: mnunez@saargo.com
-- issue: REQ-79 audit log de autenticación
-- description: registra cada intento de login, select-role, refresh y
--   logout (exitoso o no) para detección de brute-force y trazabilidad
--   forense. NO incluye passwords ni tokens (solo email_attempted +
--   metadatos). Retención: indefinida por ahora; cron de limpieza
--   futura puede tirar registros > 90 días.
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE IF NOT EXISTS auth_events (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  -- user_id nullable: en logins fallidos el usuario puede no existir.
  user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
  organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
  -- kind: login_attempt | login_success | role_selected | refreshed |
  --       logout | refresh_failed | role_select_failed
  kind            VARCHAR(40) NOT NULL,
  -- email lo que intentó el cliente (puede no coincidir con un user real).
  email_attempted VARCHAR(255),
  success         BOOLEAN NOT NULL,
  -- reason: 'invalid_credentials' | 'role_not_granted' | 'token_invalid'
  --         | 'expired' | 'user_has_no_roles' | 'ok'
  reason          VARCHAR(80),
  ip              INET,
  user_agent      TEXT,
  -- session_id si el evento corresponde a una sesión existente.
  session_id      UUID REFERENCES auth_sessions(id) ON DELETE SET NULL,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Índices: lookup por user (audit "qué hizo este user") + por
-- email+ip+ventana de tiempo (rate-limit / brute-force detection) +
-- por kind para reportes globales.
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
