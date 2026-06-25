













ALTER TABLE org_enrollment_tokens
  DROP CONSTRAINT IF EXISTS org_enrollment_tokens_organization_id_fkey;

ALTER TABLE org_enrollment_tokens
  ALTER COLUMN organization_id DROP NOT NULL;






DROP INDEX IF EXISTS org_enrollment_tokens_org_active_uniq;

CREATE UNIQUE INDEX IF NOT EXISTS org_enrollment_tokens_singleton_active_uniq
  ON org_enrollment_tokens ((TRUE))
  WHERE revoked_at IS NULL;
