-- migration: 000169_fix_project_id_fk_cascade
-- author: NunezLagos
-- issue: legacy
-- description: recrea las FK de project_id en issue_* con ON DELETE CASCADE (drop/add de constraints)
-- breaking: no
-- estimated_duration: unknown

ALTER TABLE issue_drafts            DROP CONSTRAINT issue_drafts_project_id_fkey;
-- domain-lint-ignore-next: require-not-valid-fk
ALTER TABLE issue_drafts            ADD CONSTRAINT issue_drafts_project_id_fkey            FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE issue_intake_payloads   DROP CONSTRAINT issue_intake_payloads_project_id_fkey;
-- domain-lint-ignore-next: require-not-valid-fk
ALTER TABLE issue_intake_payloads   ADD CONSTRAINT issue_intake_payloads_project_id_fkey   FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE issue_gherkin_scenarios DROP CONSTRAINT issue_gherkin_scenarios_project_id_fkey;
-- domain-lint-ignore-next: require-not-valid-fk
ALTER TABLE issue_gherkin_scenarios ADD CONSTRAINT issue_gherkin_scenarios_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE issue_tasks             DROP CONSTRAINT issue_tasks_project_id_fkey;
-- domain-lint-ignore-next: require-not-valid-fk
ALTER TABLE issue_tasks             ADD CONSTRAINT issue_tasks_project_id_fkey             FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

ALTER TABLE issue_code_references   DROP CONSTRAINT issue_code_references_project_id_fkey;
-- domain-lint-ignore-next: require-not-valid-fk
ALTER TABLE issue_code_references   ADD CONSTRAINT issue_code_references_project_id_fkey   FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;
