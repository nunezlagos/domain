-- migration: create_idempotency_keys
-- author: nunezlagos
-- issue: HU-13.4
-- description: caching de responses para keys Idempotency-Key (24h retention)
-- breaking: false
-- estimated_duration: <1s

CREATE TABLE idempotency_keys (
  id BIGSERIAL PRIMARY KEY,
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  key VARCHAR(255) NOT NULL,
  request_method VARCHAR(10) NOT NULL,
  request_path VARCHAR(500) NOT NULL,
  request_body_hash BYTEA NOT NULL,         -- sha256 del body original
  response_status SMALLINT NOT NULL,
  response_headers JSONB NOT NULL DEFAULT '{}',
  response_body BYTEA,
  expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '24 hours'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (organization_id, key)
);

CREATE INDEX idempotency_keys_expires_idx ON idempotency_keys (expires_at);

GRANT SELECT, INSERT, DELETE ON idempotency_keys TO app_user;
GRANT USAGE, SELECT ON SEQUENCE idempotency_keys_id_seq TO app_user;
GRANT ALL ON idempotency_keys TO app_admin;
GRANT ALL ON SEQUENCE idempotency_keys_id_seq TO app_admin;
