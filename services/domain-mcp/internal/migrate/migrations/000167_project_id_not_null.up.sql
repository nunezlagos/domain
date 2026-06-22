-- migration: project_id_not_null
-- author: mnunez@saargo.com
-- issue: scoping por proyecto Fase 2 — project_id OBLIGATORIO en tablas core
-- description: impone NOT NULL en project_id sobre las 7 tablas SDD-especificas a
--   las que 000161 lo agrego nullable. Precondicion: 000166 ya backfilleo los
--   derivables y borro las huerfanas, asi que ninguna fila tiene project_id NULL
--   al correr esto. El codigo (services + tools MCP) ya rechaza nil/uuid.Nil ANTES
--   del insert con ErrProjectIDRequired, asi que tras este deploy no entran filas
--   sin proyecto. SET NOT NULL escanea la tabla para validar; greenfield => tablas
--   chicas => lock instantaneo.
-- NOTA: flow_runs queda EXCLUIDA a proposito. flow_runs es dual-use: el
--   orquestador SDD setea project_id (y lo valida != Nil), pero el runner
--   generico de flows (cron, webhooks, domain_flow_run) corre flows sin proyecto
--   y debe poder insertar project_id NULL. Forzar NOT NULL ahi romperia esos
--   caminos vivos. Su scoping queda como nullable (lo llena solo el path SDD).
-- breaking: true (un caller que aun pase project_id NULL recibira un error de PG;
--   los services ya lo rechazan antes, ver Fase 2 codigo)
-- estimated_duration: <1s (validacion de columna sobre tablas casi vacias)

ALTER TABLE sdd_requirements        ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issues                  ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issue_drafts            ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issue_gherkin_scenarios ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issue_tasks             ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issue_code_references   ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issue_intake_payloads   ALTER COLUMN project_id SET NOT NULL;
