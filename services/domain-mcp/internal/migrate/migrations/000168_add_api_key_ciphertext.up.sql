-- migration: 000168_add_api_key_ciphertext
-- author: NunezLagos
-- issue: legacy
-- description: extension pgcrypto + columna key_ciphertext (bytea) en auth_api_keys
-- breaking: no
-- estimated_duration: unknown

CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE auth_api_keys ADD COLUMN key_ciphertext BYTEA;
