-- migration: rls_projects
-- author: mnunez@saargo.com
-- issue: REQ-40
-- description: RLS + FORCE en projects (defense-in-depth multi-tenant)
-- breaking: false
-- estimated_duration: <1s
--
-- Extiende el patrón de 000028/000085 a projects. App debe ejecutar
-- `SET LOCAL app.current_org_id = '<uuid>'` al inicio de cada tx vía db.WithOrgTx.
-- Sin SET LOCAL, las queries devuelven 0 rows.

ALTER TABLE projects ENABLE ROW LEVEL SECURITY;
ALTER TABLE projects FORCE ROW LEVEL SECURITY;
CREATE POLICY projects_org_isolation ON projects
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON projects TO app_user;
GRANT ALL ON projects TO app_admin;
