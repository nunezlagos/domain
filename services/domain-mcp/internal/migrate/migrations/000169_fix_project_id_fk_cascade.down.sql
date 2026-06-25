


ALTER TABLE issue_drafts            DROP CONSTRAINT issue_drafts_project_id_fkey;
ALTER TABLE issue_drafts            ADD CONSTRAINT issue_drafts_project_id_fkey            FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL;

ALTER TABLE issue_intake_payloads   DROP CONSTRAINT issue_intake_payloads_project_id_fkey;
ALTER TABLE issue_intake_payloads   ADD CONSTRAINT issue_intake_payloads_project_id_fkey   FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL;

ALTER TABLE issue_gherkin_scenarios DROP CONSTRAINT issue_gherkin_scenarios_project_id_fkey;
ALTER TABLE issue_gherkin_scenarios ADD CONSTRAINT issue_gherkin_scenarios_project_id_fkey FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL;

ALTER TABLE issue_tasks             DROP CONSTRAINT issue_tasks_project_id_fkey;
ALTER TABLE issue_tasks             ADD CONSTRAINT issue_tasks_project_id_fkey             FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL;

ALTER TABLE issue_code_references   DROP CONSTRAINT issue_code_references_project_id_fkey;
ALTER TABLE issue_code_references   ADD CONSTRAINT issue_code_references_project_id_fkey   FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL;
