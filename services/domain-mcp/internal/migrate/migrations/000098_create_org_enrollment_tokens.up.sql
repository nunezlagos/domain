






CREATE TABLE IF NOT EXISTS org_enrollment_tokens (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  token_hash          BYTEA NOT NULL,
  token_prefix        VARCHAR(20) NOT NULL,
  role_on_enroll      VARCHAR(30) NOT NULL DEFAULT 'member',
  created_by_user_id  UUID REFERENCES users(id) ON DELETE SET NULL,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at          TIMESTAMPTZ,
  CONSTRAINT org_enrollment_tokens_role_check
    CHECK (role_on_enroll IN ('owner','admin','maintainer','member','viewer'))
);


CREATE INDEX IF NOT EXISTS org_enrollment_tokens_prefix_idx
  ON org_enrollment_tokens (token_prefix)
  WHERE revoked_at IS NULL;



CREATE UNIQUE INDEX IF NOT EXISTS org_enrollment_tokens_org_active_uniq
  ON org_enrollment_tokens (organization_id)
  WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS org_enrollment_tokens_org_idx
  ON org_enrollment_tokens (organization_id, created_at DESC);

GRANT SELECT, INSERT, UPDATE ON org_enrollment_tokens TO app_user;
GRANT ALL ON org_enrollment_tokens TO app_admin;
