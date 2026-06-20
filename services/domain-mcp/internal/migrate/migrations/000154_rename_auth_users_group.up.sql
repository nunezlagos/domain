-- migration: rename_auth_users_group
-- author: mnunez@saargo.com
-- issue: REQ-42.8 (taxonomía de schema — grupo AUTH/USERS)
-- description: rename atómico de las 4 tablas auth del grupo AUTH para que
--   TODAS lleven el prefijo de su funcionalidad (auth_), arrastrando
--   sus índices, constraints y RLS policies. Es el grupo MÁS sensible:
--   sesiones/credenciales/autenticación en runtime. Riesgo en datos NULO
--   (las tablas afectadas están en 0 rows; auth_events/auth_sessions NO se tocan).
--
--   DECISIÓN CANÓNICA (REQ-42): users, roles y user_roles MANTIENEN su nombre
--   actual (NO se renombran a users_users/users_roles/users_user_roles). El
--   nombre coincide con el grupo (excepción documentada estilo Rails/Postgres):
--   `users` ES el grupo users. roles y user_roles quedan canónicas en el mismo
--   grupo. Solo se renombra el bloque auth_.
--
--   NOTA DE ALCANCE: org_enrollment_tokens→enrollment_tokens NO es parte de
--   esta migración. Ese rename pertenece a REQ-42.7 (migración 000153), que ya
--   lo aplica. Incluirlo aquí provocaría una doble-operación: 000154 fallaría
--   porque la tabla ya se llama enrollment_tokens tras 000153.
--
--   Tablas (4, todas auth_):
--     otp_codes→auth_otp_codes, api_keys→auth_api_keys,
--     secrets→auth_secrets, invitations→auth_invitations
--
--   Particularidades:
--     - SIN sequences: las tablas usan PK UUID (gen_random_uuid()).
--       NO hay ALTER SEQUENCE (a diferencia de 000146).
--     - otp_codes es la ÚNICA tabla del grupo con policy nombrada VIVA
--       (otp_codes_user_isolation, FOR ALL, USING/WITH CHECK
--       user_id = current_user_id(), creada en 000028). Sobrevivió a 000142
--       porque filtra por user_id, NO por organization_id. El ALTER TABLE
--       RENAME conserva la policy + su expresión + el FORCE RLS, PERO la
--       policy mantiene su nombre viejo: hay que ALTER POLICY ... RENAME
--       explícito. NO se recrea (sin DROP/CREATE).
--     - api_keys/secrets tienen relforcerowsecurity=t SIN policy
--       nombrada (deny-all defense-in-depth; las *_org_isolation se
--       dropearon en 000142). El RENAME conserva el flag automáticamente.
--       (users también tiene FORCE RLS, pero NO se renombra: queda canónica.)
--     - users, roles y user_roles NO se tocan en esta migración (canónicas).
--       Sus FKs y objetos quedan intactos con su nombre actual.
--
--   down: rename inverso de las 4 tablas auth + policy, también atómico.
-- breaking: false (rename de naming interno; sin pérdida de datos ni cambio
--   de API pública — el código Go se actualiza en el mismo commit)
-- estimated_duration: <1s (solo renames sobre tablas vacías; ACCESS EXCLUSIVE instantáneo)

BEGIN;

-- ===================================================================
-- GRUPO auth_
-- ===================================================================

-- 1) otp_codes → auth_otp_codes (ÚNICA con RLS policy nombrada viva)
ALTER TABLE otp_codes RENAME TO auth_otp_codes;
ALTER INDEX otp_codes_pkey            RENAME TO auth_otp_codes_pkey;
ALTER INDEX otp_codes_status_idx      RENAME TO auth_otp_codes_status_idx;
ALTER INDEX otp_codes_user_active_idx RENAME TO auth_otp_codes_user_active_idx;
-- NOTA: otp_codes_pkey es index-backed; ALTER INDEX (arriba) ya renombró el constraint.
ALTER TABLE auth_otp_codes RENAME CONSTRAINT otp_codes_user_id_fkey TO auth_otp_codes_user_id_fkey;
ALTER POLICY otp_codes_user_isolation ON auth_otp_codes RENAME TO auth_otp_codes_user_isolation;

-- 2) api_keys → auth_api_keys (FORCE RLS sin policy; mayor blast radius de código)
ALTER TABLE api_keys RENAME TO auth_api_keys;
ALTER INDEX api_keys_pkey           RENAME TO auth_api_keys_pkey;
ALTER INDEX api_keys_status_idx     RENAME TO auth_api_keys_status_idx;
ALTER INDEX api_keys_key_prefix_idx RENAME TO auth_api_keys_key_prefix_idx;
ALTER INDEX api_keys_user_id_idx    RENAME TO auth_api_keys_user_id_idx;
-- NOTA: api_keys_pkey es index-backed (ya renombrado por ALTER INDEX).
ALTER TABLE auth_api_keys RENAME CONSTRAINT api_keys_user_id_fkey TO auth_api_keys_user_id_fkey;

-- 3) secrets → auth_secrets (FORCE RLS sin policy; package Go NO se renombra)
ALTER TABLE secrets RENAME TO auth_secrets;
ALTER INDEX secrets_pkey       RENAME TO auth_secrets_pkey;
ALTER INDEX secrets_status_idx RENAME TO auth_secrets_status_idx;
-- NOTA: secrets_pkey es index-backed (ya renombrado por ALTER INDEX).
ALTER TABLE auth_secrets RENAME CONSTRAINT secrets_created_by_fkey TO auth_secrets_created_by_fkey;

-- 4) invitations → auth_invitations (sin RLS; dormant)
ALTER TABLE invitations RENAME TO auth_invitations;
ALTER INDEX invitations_pkey       RENAME TO auth_invitations_pkey;
ALTER INDEX invitations_status_idx RENAME TO auth_invitations_status_idx;
ALTER INDEX invitations_email_idx  RENAME TO auth_invitations_email_idx;
ALTER INDEX invitations_token_key  RENAME TO auth_invitations_token_key;
-- NOTA: invitations_pkey e invitations_token_key son index-backed (ya renombrados por ALTER INDEX).
ALTER TABLE auth_invitations RENAME CONSTRAINT invitations_status_check             TO auth_invitations_status_check;
ALTER TABLE auth_invitations RENAME CONSTRAINT invitations_role_check               TO auth_invitations_role_check;
ALTER TABLE auth_invitations RENAME CONSTRAINT invitations_invited_by_user_id_fkey  TO auth_invitations_invited_by_user_id_fkey;
ALTER TABLE auth_invitations RENAME CONSTRAINT invitations_accepted_user_id_fkey    TO auth_invitations_accepted_user_id_fkey;

-- ===================================================================
-- enrollment_tokens — NO se renombra aquí (es REQ-42.7 / migración 000153)
-- ===================================================================
-- org_enrollment_tokens → enrollment_tokens ya lo aplica 000153. Repetirlo
-- aquí abortaría la migración (la tabla ya tiene el nombre nuevo).

-- ===================================================================
-- GRUPO users_ — NO se renombra (decisión canónica REQ-42)
-- ===================================================================
-- users, roles y user_roles mantienen su nombre actual: el nombre coincide
-- con el grupo (excepción estilo Rails/Postgres). NO hay ALTER aquí.

COMMIT;
