-- migration: rename_org_enrollment_tokens_to_enrollment_tokens
-- author: mnunez@saargo.com
-- issue: REQ-42.7 (schema naming taxonomy — rename enrollment)
-- description: rename table `org_enrollment_tokens` → `enrollment_tokens`. El
--   prefijo `org_` era vestigial del diseño multi-tenant; en single-org el token
--   de enrolamiento es un singleton global (1 token activo a la vez vía el índice
--   parcial `..._singleton_active_uniq`). El rename preserva todos los datos y
--   referencias (PK, índices, FK a users, CHECK).
--
--   Detalles del schema real (verificados contra el catálogo):
--   - PK es UUID (gen_random_uuid()) → NO hay sequence que renombrar.
--   - NO hay RLS policy (relrowsecurity = f) → nada que ALTER POLICY.
--   - NO hay FKs entrantes → ninguna tabla vecina cambia.
--   - El pkey es índice Y constraint sobre el MISMO objeto: `ALTER INDEX` lo
--     renombra de una sola vez. NO se emite además `ALTER TABLE RENAME CONSTRAINT`
--     sobre el pkey (sería un duplicado y abortaría la migración).
--
--   Cambios:
--   1. ALTER TABLE RENAME
--   2. 4× ALTER INDEX RENAME (pkey, prefix_idx, singleton_active_uniq, status_idx)
--   3. 2× ALTER TABLE RENAME CONSTRAINT (FK created_by_user_id_fkey + CHECK role_check)
--
--   down: RENAME reverso (atómico).
-- breaking: false (cambio de naming interno; API pública no afectada)
-- estimated_duration: <1s (tabla con 0 filas; RENAME es operación de catálogo O(1))

BEGIN;

ALTER TABLE org_enrollment_tokens RENAME TO enrollment_tokens;

ALTER INDEX org_enrollment_tokens_pkey                  RENAME TO enrollment_tokens_pkey;
ALTER INDEX org_enrollment_tokens_prefix_idx            RENAME TO enrollment_tokens_prefix_idx;
ALTER INDEX org_enrollment_tokens_singleton_active_uniq RENAME TO enrollment_tokens_singleton_active_uniq;
ALTER INDEX org_enrollment_tokens_status_idx            RENAME TO enrollment_tokens_status_idx;

ALTER TABLE enrollment_tokens
  RENAME CONSTRAINT org_enrollment_tokens_created_by_user_id_fkey
  TO enrollment_tokens_created_by_user_id_fkey;
ALTER TABLE enrollment_tokens
  RENAME CONSTRAINT org_enrollment_tokens_role_check
  TO enrollment_tokens_role_check;

COMMIT;
