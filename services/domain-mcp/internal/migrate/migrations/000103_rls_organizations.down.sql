DROP POLICY IF EXISTS organizations_self_isolation ON organizations;
ALTER TABLE organizations DISABLE ROW LEVEL SECURITY;
