
























BEGIN;

ALTER TABLE gherkin_scenarios RENAME TO issue_gherkin_scenarios;

ALTER INDEX gherkin_scenarios_pkey       RENAME TO issue_gherkin_scenarios_pkey;
ALTER INDEX gherkin_hu_id_idx            RENAME TO issue_gherkin_scenarios_issue_id_idx;
ALTER INDEX gherkin_scenarios_status_idx RENAME TO issue_gherkin_scenarios_status_idx;

ALTER TABLE issue_gherkin_scenarios
  RENAME CONSTRAINT gherkin_scenarios_hu_id_fkey
  TO issue_gherkin_scenarios_issue_id_fkey;

COMMIT;
