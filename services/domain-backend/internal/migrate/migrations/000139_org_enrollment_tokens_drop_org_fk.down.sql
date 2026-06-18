-- Revertir: restaurar FK a organizations(id) ON DELETE CASCADE y el UNIQUE
-- INDEX partial sobre (organization_id) WHERE revoked_at IS NULL.
-- Nota: falla si hay más de 1 fila con organization_id NULL y revoked_at IS NULL
-- (el UNIQUE INDEX original era per-org).
DROP INDEX IF EXISTS org_enrollment_tokens_singleton_active_uniq;

CREATE UNIQUE INDEX IF NOT EXISTS org_enrollment_tokens_org_active_uniq
  ON org_enrollment_tokens (organization_id)
  WHERE revoked_at IS NULL;

ALTER TABLE org_enrollment_tokens
  ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE org_enrollment_tokens
  ADD CONSTRAINT org_enrollment_tokens_organization_id_fkey
  FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;
