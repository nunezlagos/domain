-- migration: 000271_prompts_user_modified
-- author: nunezlagos
-- issue: DOMAINSERV-27
-- description: agrega is_user_modified a prompts para que los 4 prompt seeders
--   (first_response/analysis/triage/wizard_formulator) no pisen ediciones del
--   usuario en el próximo bump de Version(). Expand seguro: columna con DEFAULT,
--   sin backfill ni DML sobre datos de usuario.
-- breaking: no
-- duration: <1s
ALTER TABLE prompts
  ADD COLUMN IF NOT EXISTS is_user_modified BOOLEAN NOT NULL DEFAULT FALSE;
