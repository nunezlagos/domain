DO $$
DECLARE
  t TEXT;
BEGIN
  FOREACH t IN ARRAY ARRAY[
    'issues','issue_drafts','issue_draft_steps_log','issue_gherkin_scenarios',
    'issue_intake_payloads','issue_tasks','issue_code_references',
    'sdd_requirements','sdd_proposals','sdd_designs',
    'tdd_verifications','tdd_verification_results','tdd_sabotage_records'
  ]
  LOOP
    IF to_regclass('public.' || t) IS NOT NULL THEN
      EXECUTE format('COMMENT ON TABLE %I IS NULL', t);
    END IF;
  END LOOP;
END$$;
