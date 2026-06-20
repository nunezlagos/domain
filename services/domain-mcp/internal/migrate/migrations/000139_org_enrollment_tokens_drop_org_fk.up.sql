-- migration: org_enrollment_tokens_drop_org_fk
-- author: mnunez@saargo.com
-- issue: REQ-21.6 (Fase B — per-consumer cleanup)
-- description: org_enrollment_tokens tiene organization_id NOT NULL con FK a
--   organizations(id) ON DELETE CASCADE, lo cual bloquea DROP TABLE organizations
--   en Fase C. Sacamos la FK (y el cascade) y dejamos organization_id nullable.
--   El código de enrollment/service.Rotate/Revoke/Enroll/GetMetadata ya NO
--   escribe ni filtra por organization_id (issue-37.1 single-org global).
--   El UNIQUE INDEX partial sobre (organization_id) WHERE revoked_at IS NULL
--   se reemplaza por un CHECK que garantiza 1 token activo a la vez (defense
--   in depth: la app ya lo enforce por tx).
-- breaking: false (código no usa organization_id)
-- estimated_duration: <1s

ALTER TABLE org_enrollment_tokens
  DROP CONSTRAINT IF EXISTS org_enrollment_tokens_organization_id_fkey;

ALTER TABLE org_enrollment_tokens
  ALTER COLUMN organization_id DROP NOT NULL;

-- El UNIQUE INDEX partial (organization_id) WHERE revoked_at IS NULL ya no
-- aplica: sin organization_id, no se puede garantizar "1 token activo por org".
-- Se reemplaza por una invariante más simple: como mucho 1 token activo total
-- (single-org global). El código (Rotate) ya lo enforce por tx; la DB solo
-- ofrece defense in depth via un partial UNIQUE sobre un literal.
DROP INDEX IF EXISTS org_enrollment_tokens_org_active_uniq;

CREATE UNIQUE INDEX IF NOT EXISTS org_enrollment_tokens_singleton_active_uniq
  ON org_enrollment_tokens ((TRUE))
  WHERE revoked_at IS NULL;
