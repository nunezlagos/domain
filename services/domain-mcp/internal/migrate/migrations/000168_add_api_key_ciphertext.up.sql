














CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE auth_api_keys ADD COLUMN key_ciphertext BYTEA;
