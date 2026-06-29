DROP INDEX IF EXISTS auth_api_keys_environment_idx;

ALTER TABLE auth_api_keys
  DROP COLUMN IF EXISTS environment;
