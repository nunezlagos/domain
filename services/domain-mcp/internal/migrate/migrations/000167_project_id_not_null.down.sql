-- down: revierte el NOT NULL a nullable (vuelve al estado post-000161).
--   No restaura las filas borradas por 000166 (eso es irreversible), solo
--   afloja la restriccion de columna por si hay que volver a aceptar NULL
--   temporalmente (rollback de la Fase 2). Orden inverso al up.
ALTER TABLE issue_intake_payloads   ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE issue_code_references   ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE issue_tasks             ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE issue_gherkin_scenarios ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE issue_drafts            ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE issues                  ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE sdd_requirements        ALTER COLUMN project_id DROP NOT NULL;
