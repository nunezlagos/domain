-- migration: create_clients
-- author: mnunez@saargo.com
-- issue: REQ-39
-- description: tabla clients (cuentas/empresas) con FK a organization + soft delete + status
-- breaking: false
-- estimated_duration: <1s
--
-- Clients representa cuentas/empresas externas que la organización gestiona como contraparte
-- (clientes finales, partners, contratantes). Aislado por organization_id (multi-tenant).
-- Slug único per-org. Status acotado por CHECK. metadata jsonb para extensibilidad.

CREATE TABLE clients (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  name VARCHAR(255) NOT NULL,
  slug VARCHAR(100) NOT NULL,
  tax_id VARCHAR(50),
  contact_email VARCHAR(255),
  contact_phone VARCHAR(50),
  address TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  status VARCHAR(20) NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'inactive', 'archived')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (organization_id, slug)
);

CREATE TRIGGER set_updated_at_clients
  BEFORE UPDATE ON clients
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX clients_organization_id_idx ON clients (organization_id) WHERE deleted_at IS NULL;

-- RLS multi-tenant: defense-in-depth contra bugs RBAC app.
-- FORCE para que aplique también al owner; app_admin (BYPASSRLS) sigue saltando.
ALTER TABLE clients ENABLE ROW LEVEL SECURITY;
ALTER TABLE clients FORCE ROW LEVEL SECURITY;
CREATE POLICY clients_org_isolation ON clients
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

-- Re-grants explícitos: ALTER DEFAULT PRIVILEGES de 000025 solo aplica a tablas
-- creadas por app_migrator; garantizamos los grants aquí (misma razón que 000028/000085).
GRANT SELECT, INSERT, UPDATE, DELETE ON clients TO app_user;
GRANT ALL ON clients TO app_admin;
