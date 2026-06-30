-- migration: 000164_add_api_key_plaintext
-- author: NunezLagos
-- issue: legacy
-- description: columna key_plaintext en auth_api_keys
-- breaking: no
-- estimated_duration: unknown

ALTER TABLE auth_api_keys ADD COLUMN key_plaintext TEXT;
