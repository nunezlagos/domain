DROP POLICY IF EXISTS projects_org_isolation ON projects;
ALTER TABLE projects DISABLE ROW LEVEL SECURITY;
