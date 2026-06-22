-- migration: add_api_key_ciphertext
-- author: mnunez@saargo.com
-- issue: cifrado at-rest de API keys con pgcrypto (endurecer mig 000164)
-- description: habilita la extension pgcrypto y agrega key_ciphertext (BYTEA) a
--   auth_api_keys. La key en claro se persiste cifrada con pgp_sym_encrypt usando
--   una passphrase simetrica (DOMAIN_FIELD_ENC_KEY, en el env de ambos servicios).
--   El detalle del dashboard la muestra descifrada con pgp_sym_decrypt. key_plaintext
--   queda para fallback de keys viejas (creadas antes de esta mig) pero las NUEVAS
--   ya no se escriben en claro. La columna es nullable: las filas existentes la
--   tienen en NULL hasta re-cifrarse o rotarse (ver nota de backfill mas abajo).
--   No se backfillea aqui: la migracion NO tiene la passphrase. Esas keys de test
--   se limpian aparte; mientras tanto el dashboard cae al key_plaintext viejo.
-- breaking: false
-- estimated_duration: <1s

CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE auth_api_keys ADD COLUMN key_ciphertext BYTEA;
