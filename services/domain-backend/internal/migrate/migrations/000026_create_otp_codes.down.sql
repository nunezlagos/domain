DROP TABLE IF EXISTS otp_codes CASCADE;
DROP INDEX IF EXISTS users_rut_active_idx;
ALTER TABLE users
  DROP COLUMN IF EXISTS rut,
  DROP COLUMN IF EXISTS last_organization_id,
  DROP COLUMN IF EXISTS last_login_at;
