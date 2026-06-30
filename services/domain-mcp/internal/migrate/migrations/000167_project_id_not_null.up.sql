-- migration: 000167_project_id_not_null
-- author: NunezLagos
-- issue: legacy
-- description: fija project_id como NOT NULL en sdd_requirements, issues e issue_* tras el backfill
-- breaking: no
-- estimated_duration: unknown

ALTER TABLE sdd_requirements        ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issues                  ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issue_drafts            ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issue_gherkin_scenarios ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issue_tasks             ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issue_code_references   ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE issue_intake_payloads   ALTER COLUMN project_id SET NOT NULL;
