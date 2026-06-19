-- migration: rename_gherkin_scenarios_to_issue_gherkin_scenarios (down)
-- author: mnunez@saargo.com
-- issue: REQ-42.6 (schema naming taxonomy — dominio issue)
-- description: rollback — rename table `issue_gherkin_scenarios` →
--   `gherkin_scenarios` y restaura los nombres legacy de índices y
--   constraint (gherkin_hu_id_idx, gherkin_scenarios_hu_id_fkey).
--   Reverso simétrico de la up, misma tx.
-- breaking: false
-- estimated_duration: <1s

BEGIN;

ALTER TABLE issue_gherkin_scenarios RENAME TO gherkin_scenarios;

ALTER INDEX issue_gherkin_scenarios_pkey          RENAME TO gherkin_scenarios_pkey;
ALTER INDEX issue_gherkin_scenarios_issue_id_idx  RENAME TO gherkin_hu_id_idx;
ALTER INDEX issue_gherkin_scenarios_status_idx    RENAME TO gherkin_scenarios_status_idx;

ALTER TABLE gherkin_scenarios
  RENAME CONSTRAINT issue_gherkin_scenarios_issue_id_fkey
  TO gherkin_scenarios_hu_id_fkey;

COMMIT;
