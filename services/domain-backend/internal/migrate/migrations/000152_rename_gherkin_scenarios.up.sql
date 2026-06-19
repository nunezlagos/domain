-- migration: rename_gherkin_scenarios_to_issue_gherkin_scenarios
-- author: mnunez@saargo.com
-- issue: REQ-42.6 (schema naming taxonomy — dominio issue)
-- description: rename table `gherkin_scenarios` → `issue_gherkin_scenarios`.
--   La tabla guarda los criterios de aceptación BDD (feature/scenario/
--   given/when/then) atados al issue vía issue_id; pertenece al grupo
--   `issue_` de la taxonomía. Se aprovecha el rename para alinear los
--   objetos legacy de la era HU (gherkin_hu_id_idx, *_hu_id_fkey) a la
--   columna real issue_id + prefijo issue_.
--
--   Cambios (una sola tx, RENAME metadata-only, sin reescritura):
--   1. ALTER TABLE gherkin_scenarios RENAME TO issue_gherkin_scenarios
--   2. ALTER INDEX x3 (pkey, gherkin_hu_id_idx legacy, status)
--   3. ALTER TABLE RENAME CONSTRAINT del fkey legacy hu_id → issue_id
--
--   NO se toca: sequence (PK UUID gen_random_uuid, no existe), trigger
--   genérico trg_set_updated_at (sobrevive por OID), FK saliente
--   issue_id → issues (Postgres mantiene la referencia por OID), RLS
--   (la tabla no tiene policies). El pkey constraint se renombra junto
--   con su índice (no se emite RENAME CONSTRAINT separado).
--
--   down: RENAME reverso simétrico (atómico).
-- breaking: false (cambio de naming interno; API pública no afectada)
-- estimated_duration: <1s

BEGIN;

ALTER TABLE gherkin_scenarios RENAME TO issue_gherkin_scenarios;

ALTER INDEX gherkin_scenarios_pkey       RENAME TO issue_gherkin_scenarios_pkey;
ALTER INDEX gherkin_hu_id_idx            RENAME TO issue_gherkin_scenarios_issue_id_idx;
ALTER INDEX gherkin_scenarios_status_idx RENAME TO issue_gherkin_scenarios_status_idx;

ALTER TABLE issue_gherkin_scenarios
  RENAME CONSTRAINT gherkin_scenarios_hu_id_fkey
  TO issue_gherkin_scenarios_issue_id_fkey;

COMMIT;
