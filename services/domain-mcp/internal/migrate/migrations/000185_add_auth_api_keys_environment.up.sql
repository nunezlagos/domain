ALTER TABLE auth_api_keys
  ADD COLUMN IF NOT EXISTS environment text NOT NULL DEFAULT 'live';

CREATE INDEX IF NOT EXISTS auth_api_keys_environment_idx
  ON auth_api_keys (environment)
  WHERE revoked_at IS NULL;
