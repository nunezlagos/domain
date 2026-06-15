DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
ALTER TABLE users
  DROP COLUMN IF EXISTS password_set_at,
  DROP COLUMN IF EXISTS password_hash;
