ALTER TABLE issue_intake_payloads   DROP COLUMN IF EXISTS project_id;
ALTER TABLE issue_code_references   DROP COLUMN IF EXISTS project_id;
ALTER TABLE issue_tasks             DROP COLUMN IF EXISTS project_id;
ALTER TABLE issue_gherkin_scenarios DROP COLUMN IF EXISTS project_id;
ALTER TABLE issue_drafts            DROP COLUMN IF EXISTS project_id;
ALTER TABLE flow_runs               DROP COLUMN IF EXISTS project_id;
ALTER TABLE issues                  DROP COLUMN IF EXISTS project_id;
ALTER TABLE sdd_requirements        DROP COLUMN IF EXISTS project_id;
