BEGIN;

DROP POLICY IF EXISTS auth_otp_codes_user_isolation ON auth_otp_codes;

ALTER TABLE IF EXISTS auth_otp_codes DISABLE ROW LEVEL SECURITY;

DROP TABLE IF EXISTS auth_otp_codes CASCADE;

REVOKE ALL ON auth_otp_codes FROM app_user;
REVOKE ALL ON auth_otp_codes FROM app_admin;

COMMIT;