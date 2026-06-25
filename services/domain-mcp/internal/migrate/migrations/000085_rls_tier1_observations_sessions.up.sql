




















ALTER TABLE observations ENABLE ROW LEVEL SECURITY;
ALTER TABLE observations FORCE ROW LEVEL SECURITY;
CREATE POLICY observations_org_isolation ON observations
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());


ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions FORCE ROW LEVEL SECURITY;
CREATE POLICY sessions_org_isolation ON sessions
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());


GRANT SELECT, INSERT, UPDATE, DELETE ON observations, sessions TO app_user;
GRANT ALL ON observations, sessions TO app_admin;
