-- migration: 000276_auth_hmac_nonces
-- author: nunezlagos
-- issue: DOMAINSERV-129
-- description: soporte de anti-replay para la autenticación por firma HMAC.
--   Con el scheme DOMAIN-HMAC-SHA256 la API key deja de viajar en claro, pero
--   una firma capturada sigue siendo reusable si no se quema el nonce: la
--   ventana de ±5 min sobre el ts sola no alcanza. Esta tabla es la primitiva
--   atómica de quemado (INSERT ... ON CONFLICT DO NOTHING sobre la PK
--   compuesta): si el INSERT no afecta filas, ese (key, nonce) ya se usó y la
--   request es un replay. NO lleva RLS (mismo criterio que auth_events /
--   auth_sessions): es infraestructura de auth, no dato de tenant, y el
--   middleware la escribe antes de existir el SET LOCAL app.current_org_id.
--   Las filas se podan solas dentro del propio statement de quemado (ver
--   store_hmac.go): fuera de la ventana un nonce ya no sirve para nada.
-- breaking: no
-- estimated_duration: <1s

CREATE TABLE IF NOT EXISTS auth_hmac_nonces (
  api_key_id UUID NOT NULL REFERENCES auth_api_keys(id) ON DELETE CASCADE,
  nonce      TEXT NOT NULL CHECK (char_length(nonce) BETWEEN 1 AND 128),
  -- ts que venía firmado en el header; la poda de vencidos se ordena por acá
  signed_at  TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (api_key_id, nonce)
);

CREATE INDEX IF NOT EXISTS auth_hmac_nonces_signed_at_idx
  ON auth_hmac_nonces (signed_at);

GRANT SELECT, INSERT, DELETE ON auth_hmac_nonces TO app_user;
GRANT ALL ON auth_hmac_nonces TO app_admin;
