-- migration: 000161_add_project_id_scoping
-- author: NunezLagos
-- issue: legacy
-- description: agrega columna project_id (nullable) para aislar por proyecto en sdd_requirements, issues, flow_runs e issue_* + indices
-- breaking: no
-- estimated_duration: unknown

ALTER TABLE sdd_requirements ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE issues           ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE flow_runs        ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;


ALTER TABLE issue_drafts            ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE issue_gherkin_scenarios ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE issue_tasks             ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE issue_code_references   ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE issue_intake_payloads   ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE SET NULL;





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
