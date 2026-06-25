






CREATE TABLE api_keys (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key_hash BYTEA NOT NULL,
  key_prefix VARCHAR(20) NOT NULL,
  name VARCHAR(255) NOT NULL,
  last_used_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (organization_id, name)
);

CREATE TRIGGER set_updated_at_api_keys
  BEFORE UPDATE ON api_keys
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX api_keys_key_prefix_idx ON api_keys (key_prefix) WHERE revoked_at IS NULL;
CREATE INDEX api_keys_user_id_idx ON api_keys (user_id) WHERE revoked_at IS NULL;
