-- migration: rls_organizations
-- author: mnunez@saargo.com
-- issue: REQ-40
-- description: RLS + FORCE en organizations (defense-in-depth multi-tenant)
-- breaking: false
-- estimated_duration: <1s
--
-- organizations es la tabla root: el filtro es por id (no organization_id).
-- Una org solo puede verse a sí misma cuando app.current_org_id coincide.
-- app_admin (BYPASSRLS) sigue listando todas las orgs (admin UI, billing, etc).

ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE organizations FORCE ROW LEVEL SECURITY;
CREATE POLICY organizations_self_isolation ON organizations
  FOR ALL TO PUBLIC
  USING (id = current_org_id())
  WITH CHECK (id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON organizations TO app_user;
GRANT ALL ON organizations TO app_admin;
