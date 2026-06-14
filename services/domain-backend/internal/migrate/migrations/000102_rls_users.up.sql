-- migration: rls_users
-- author: mnunez@saargo.com
-- issue: REQ-40
-- description: RLS + FORCE en users (defense-in-depth multi-tenant)
-- breaking: false
-- estimated_duration: <1s
--
-- Filtro por organization_id = current_org_id(). App debe setear
-- app.current_org_id en cada tx (db.WithOrgTx). app_admin (BYPASSRLS)
-- mantiene acceso para login/bootstrap y batch jobs.

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;
CREATE POLICY users_org_isolation ON users
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON users TO app_user;
GRANT ALL ON users TO app_admin;
