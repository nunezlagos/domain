

DROP POLICY IF EXISTS api_keys_org_isolation ON api_keys;
ALTER TABLE api_keys DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS activity_log_org_isolation ON activity_log;
ALTER TABLE activity_log DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS otp_codes_user_isolation ON otp_codes;
ALTER TABLE otp_codes DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS audit_log_org_insert ON audit_log;
DROP POLICY IF EXISTS audit_log_org_isolation ON audit_log;
ALTER TABLE audit_log DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS secrets_org_isolation ON secrets;
ALTER TABLE secrets DISABLE ROW LEVEL SECURITY;

DROP FUNCTION IF EXISTS current_user_id();
DROP FUNCTION IF EXISTS current_org_id();
