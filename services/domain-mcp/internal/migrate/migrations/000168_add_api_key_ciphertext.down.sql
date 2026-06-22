-- Revierte solo la columna. NO se dropea la extension pgcrypto: puede estar en
-- uso por otras tablas/columnas (ej. secrets), dropearla romperia esos usos.
ALTER TABLE auth_api_keys DROP COLUMN IF EXISTS key_ciphertext;
