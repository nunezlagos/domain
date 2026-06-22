-- migration: fix_project_id_fk_cascade
-- author: mnunez@saargo.com
-- issue: bug latente de Fase 2 — FK project_id ON DELETE SET NULL vs columna NOT NULL
-- description: la mig 000161 creo las FK project_id de estas 5 tablas como
--   ON DELETE SET NULL; la mig 000167 (Fase 2) hizo project_id NOT NULL. Esa
--   combinacion es contradictoria: al borrar un proyecto, la FK intenta poner
--   project_id = NULL en las filas dependientes y viola el NOT NULL -> el DELETE
--   del proyecto FALLA. Como project_id ahora es obligatorio, lo correcto es que
--   esas filas se borren junto con su proyecto: cambiamos SET NULL -> CASCADE.
--   (flow_runs/issues/sdd_requirements ya eran CASCADE; issue_drafts e
--   issue_intake_payloads tambien aplican porque su columna quedo NOT NULL.)
-- breaking: false (cambia semantica de borrado en cascada, no el shape)
-- estimated_duration: <1s (recrea constraints sobre tablas chicas)

ALTER TABLE issue_drafts            DROP CONSTRAINT issue_drafts_project_id_fkey;
ALTER TABLE issue_drafts            ADD CONSTRAINT issue_drafts_project_id_fkey            FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE issue_intake_payloads   DROP CONSTRAINT issue_intake_payloads_project_id_fkey;
ALTER TABLE issue_intake_payloads   ADD CONSTRAINT issue_intake_payloads_project_id_fkey   FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE issue_gherkin_scenarios DROP CONSTRAINT issue_gherkin_scenarios_project_id_fkey;
ALTER TABLE issue_gherkin_scenarios ADD CONSTRAINT issue_gherkin_scenarios_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE issue_tasks             DROP CONSTRAINT issue_tasks_project_id_fkey;
ALTER TABLE issue_tasks             ADD CONSTRAINT issue_tasks_project_id_fkey             FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE issue_code_references   DROP CONSTRAINT issue_code_references_project_id_fkey;
ALTER TABLE issue_code_references   ADD CONSTRAINT issue_code_references_project_id_fkey   FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;
