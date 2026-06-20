-- Revertir: restaurar tabla organizations + función current_org_id().
-- Esto es ESENCIAL para hacer rollback de Fase C si algo sale mal.
-- En roundtrip (DB fresca) no hay datos que restaurar — el reverse deja
-- la DB en un estado consistente con las migraciones previas a Fase C.
-- En DB con datos, las foreign keys desde satellites NO se restauran
-- automáticamente (esos DROPs en 000140 son parte de la destrucción).
-- Para restore completo: pgBackRest (ver docs/runbooks/restore.md).

-- Recrear tabla organizations con el schema original de 000002
CREATE TABLE IF NOT EXISTS organizations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(255) NOT NULL,
  slug VARCHAR(255) UNIQUE NOT NULL,
  settings JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ
);

CREATE TRIGGER IF NOT EXISTS set_updated_at_organizations
  BEFORE UPDATE ON organizations
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX IF NOT EXISTS organizations_slug_active_idx ON organizations (slug) WHERE deleted_at IS NULL;

-- Recrear función current_org_id()
CREATE OR REPLACE FUNCTION current_org_id() RETURNS UUID AS $$
  SELECT NULLIF(current_setting('app.current_org_id', true), '')::UUID;
$$ LANGUAGE SQL STABLE;

GRANT EXECUTE ON FUNCTION current_org_id() TO app_user, app_admin;
