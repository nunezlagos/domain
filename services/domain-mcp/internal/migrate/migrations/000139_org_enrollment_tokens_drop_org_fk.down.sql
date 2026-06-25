



DROP INDEX IF EXISTS org_enrollment_tokens_singleton_active_uniq;

CREATE UNIQUE INDEX IF NOT EXISTS org_enrollment_tokens_org_active_uniq
  ON org_enrollment_tokens (organization_id)
  WHERE revoked_at IS NULL;

ALTER TABLE org_enrollment_tokens
  ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE org_enrollment_tokens
  ADD CONSTRAINT org_enrollment_tokens_organization_id_fkey
  FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;
