



ALTER TABLE issue_intake_payloads   ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE issue_code_references   ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE issue_tasks             ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE issue_gherkin_scenarios ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE issue_drafts            ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE issues                  ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE sdd_requirements        ALTER COLUMN project_id DROP NOT NULL;
