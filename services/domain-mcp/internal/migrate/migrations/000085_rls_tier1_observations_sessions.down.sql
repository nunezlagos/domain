


DROP POLICY IF EXISTS observations_org_isolation ON observations;
ALTER TABLE observations DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS sessions_org_isolation ON sessions;
ALTER TABLE sessions DISABLE ROW LEVEL SECURITY;
