















CREATE OR REPLACE FUNCTION current_org_id() RETURNS UUID AS $$
BEGIN
  RETURN nullif(current_setting('app.current_org_id', true), '')::uuid;
EXCEPTION WHEN OTHERS THEN
  RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;






ALTER TABLE secrets ENABLE ROW LEVEL SECURITY;
ALTER TABLE secrets FORCE ROW LEVEL SECURITY;
CREATE POLICY secrets_org_isolation ON secrets
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());


ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_log FORCE ROW LEVEL SECURITY;
CREATE POLICY audit_log_org_isolation ON audit_log
  FOR SELECT TO PUBLIC
  USING (organization_id = current_org_id() OR organization_id IS NULL);

CREATE POLICY audit_log_org_insert ON audit_log
  FOR INSERT TO PUBLIC
  WITH CHECK (true);




CREATE OR REPLACE FUNCTION current_user_id() RETURNS UUID AS $$
BEGIN
  RETURN nullif(current_setting('app.current_user_id', true), '')::uuid;
EXCEPTION WHEN OTHERS THEN
  RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

ALTER TABLE otp_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE otp_codes FORCE ROW LEVEL SECURITY;
CREATE POLICY otp_codes_user_isolation ON otp_codes
  FOR ALL TO PUBLIC
  USING (user_id = current_user_id())
  WITH CHECK (user_id = current_user_id());


ALTER TABLE activity_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE activity_log FORCE ROW LEVEL SECURITY;
CREATE POLICY activity_log_org_isolation ON activity_log
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());


ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY api_keys_org_isolation ON api_keys
  FOR ALL TO PUBLIC
  USING (organization_id = current_org_id())
  WITH CHECK (organization_id = current_org_id());




GRANT SELECT, INSERT, UPDATE, DELETE ON secrets, otp_codes, activity_log, api_keys TO app_user;
GRANT SELECT, INSERT ON audit_log TO app_user;
GRANT ALL ON secrets, audit_log, otp_codes, activity_log, api_keys TO app_admin;
