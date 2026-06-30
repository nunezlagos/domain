







ALTER TABLE users
  ADD COLUMN IF NOT EXISTS rut VARCHAR(12) UNIQUE,
  ADD COLUMN IF NOT EXISTS last_organization_id UUID REFERENCES organizations(id),
  ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;




CREATE INDEX IF NOT EXISTS users_rut_active_idx
  ON users (rut) WHERE rut IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE otp_codes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  code_hash BYTEA NOT NULL,                         -- bcrypt(code)
  attempts SMALLINT NOT NULL DEFAULT 0,
  max_attempts SMALLINT NOT NULL DEFAULT 5,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  ip_address VARCHAR(45),
  user_agent VARCHAR(500),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX otp_codes_user_active_idx
  ON otp_codes (user_id, created_at DESC) WHERE used_at IS NULL;
