BEGIN;

CREATE TABLE IF NOT EXISTS auth_otp_codes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash BYTEA NOT NULL,
  attempts SMALLINT NOT NULL DEFAULT 0,
  max_attempts SMALLINT NOT NULL DEFAULT 5,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  ip_address VARCHAR(45),
  user_agent VARCHAR(500),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS auth_otp_codes_user_active_idx
  ON auth_otp_codes (user_id, created_at DESC) WHERE used_at IS NULL;

ALTER TABLE auth_otp_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth_otp_codes FORCE ROW LEVEL SECURITY;

CREATE POLICY auth_otp_codes_user_isolation ON auth_otp_codes
  FOR ALL TO PUBLIC
  USING (user_id = current_user_id())
  WITH CHECK (user_id = current_user_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON auth_otp_codes TO app_user;
GRANT ALL ON auth_otp_codes TO app_admin;

COMMIT;