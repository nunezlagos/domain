









BEGIN;

ALTER TABLE issue_gherkin_scenarios RENAME TO gherkin_scenarios;

ALTER INDEX issue_gherkin_scenarios_pkey          RENAME TO gherkin_scenarios_pkey;
ALTER INDEX issue_gherkin_scenarios_issue_id_idx  RENAME TO gherkin_hu_id_idx;
ALTER INDEX issue_gherkin_scenarios_status_idx    RENAME TO gherkin_scenarios_status_idx;

ALTER TABLE gherkin_scenarios
  RENAME CONSTRAINT issue_gherkin_scenarios_issue_id_fkey
  TO gherkin_scenarios_hu_id_fkey;

COMMIT;
