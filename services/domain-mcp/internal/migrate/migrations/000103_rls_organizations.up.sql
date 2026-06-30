










ALTER TABLE organizations ENABLE ROW LEVEL SECURITY;
ALTER TABLE organizations FORCE ROW LEVEL SECURITY;
CREATE POLICY organizations_self_isolation ON organizations
  FOR ALL TO PUBLIC
  USING (id = current_org_id())
  WITH CHECK (id = current_org_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON organizations TO app_user;
GRANT ALL ON organizations TO app_admin;
