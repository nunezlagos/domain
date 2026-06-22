-- migration: add_project_id_scoping
-- author: mnunez@saargo.com
-- issue: scoping por proyecto — documentos SDD/TDD + flow_runs
-- description: agrega project_id (nullable) a la raiz de la cadena SDD
--   (sdd_requirements), a issues y a las tablas issue-facing, y a flow_runs.
--   Nullable-first: el backfill/NOT NULL queda para una ola posterior, una vez
--   que el codigo escribe project_id end-to-end. sdd_proposals/sdd_designs y
--   tdd_* derivan project_id via JOIN (no llevan columna). Sin organization_id
--   (org-less). Indices planos: prod greenfield (0 filas), instantaneos.
-- breaking: false
-- estimated_duration: <1s (ADD COLUMN nullable, sin reescritura)

-- Raiz + ownership directo: CASCADE (borrar proyecto borra su cadena SDD).
ALTER TABLE sdd_requirements ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE issues           ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE flow_runs        ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;

-- Tablas issue-facing satelite: SET NULL (la traza igual cae por su issue_id).
ALTER TABLE issue_drafts            ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE issue_gherkin_scenarios ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE issue_tasks             ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE issue_code_references   ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE issue_intake_payloads   ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE SET NULL;

-- Indices planos a proposito: las tablas estan vacias (greenfield), el lock es
-- instantaneo. CONCURRENTLY no puede usarse aca (golang-migrate corre el archivo
-- como multi-statement -> transaccion implicita -> CONCURRENTLY abortaria).
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX sdd_requirements_project_id_idx     ON sdd_requirements(project_id);
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX issues_project_id_idx               ON issues(project_id);
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX flow_runs_project_id_idx            ON flow_runs(project_id);
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX issue_drafts_project_id_idx         ON issue_drafts(project_id);
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX issue_gherkin_scenarios_project_id_idx ON issue_gherkin_scenarios(project_id);
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX issue_tasks_project_id_idx          ON issue_tasks(project_id);
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX issue_code_references_project_id_idx ON issue_code_references(project_id);
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX issue_intake_payloads_project_id_idx ON issue_intake_payloads(project_id);
