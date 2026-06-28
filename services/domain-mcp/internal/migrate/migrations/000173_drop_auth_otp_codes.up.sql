-- migration: 000173_drop_auth_otp_codes
-- author: NunezLagos
-- issue: legacy
-- description: elimina la tabla auth_otp_codes (policy, RLS, tabla y grants)
-- breaking: yes
-- estimated_duration: unknown

DROP POLICY IF EXISTS auth_otp_codes_user_isolation ON auth_otp_codes;

ALTER TABLE IF EXISTS auth_otp_codes DISABLE ROW LEVEL SECURITY;

REVOKE ALL ON auth_otp_codes FROM app_user;
REVOKE ALL ON auth_otp_codes FROM app_admin;

DROP TABLE IF EXISTS auth_otp_codes CASCADE;