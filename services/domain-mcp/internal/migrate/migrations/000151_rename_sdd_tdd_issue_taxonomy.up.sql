-- migration: 000151_rename_sdd_tdd_issue_taxonomy
-- author: NunezLagos
-- issue: legacy
-- description: renombra tablas a la taxonomia sdd_/tdd_/issue_ (multiples ALTER TABLE RENAME)
-- breaking: yes
-- estimated_duration: unknown

BEGIN;




ALTER TABLE IF EXISTS requirements RENAME TO sdd_requirements;

ALTER INDEX IF EXISTS requirements_pkey            RENAME TO sdd_requirements_pkey;
ALTER INDEX IF EXISTS requirements_parent_id_idx   RENAME TO sdd_requirements_parent_id_idx;
ALTER INDEX IF EXISTS requirements_priority_idx    RENAME TO sdd_requirements_priority_idx;
ALTER INDEX IF EXISTS requirements_slug_idx        RENAME TO sdd_requirements_slug_idx;
ALTER INDEX IF EXISTS requirements_status_idx      RENAME TO sdd_requirements_status_idx;

ALTER TABLE sdd_requirements RENAME CONSTRAINT requirements_parent_id_fkey TO sdd_requirements_parent_id_fkey;





ALTER TABLE IF EXISTS proposals RENAME TO sdd_proposals;

ALTER INDEX IF EXISTS proposals_pkey                 RENAME TO sdd_proposals_pkey;
ALTER INDEX IF EXISTS proposals_hu_id_version_key    RENAME TO sdd_proposals_issue_id_version_key;
ALTER INDEX IF EXISTS proposals_status_idx           RENAME TO sdd_proposals_status_idx;



ALTER TABLE sdd_proposals RENAME CONSTRAINT proposals_hu_id_fkey        TO sdd_proposals_issue_id_fkey;






ALTER TABLE IF EXISTS designs RENAME TO sdd_designs;

ALTER INDEX IF EXISTS designs_pkey               RENAME TO sdd_designs_pkey;
ALTER INDEX IF EXISTS designs_hu_id_version_key  RENAME TO sdd_designs_issue_id_version_key;
ALTER INDEX IF EXISTS designs_status_idx         RENAME TO sdd_designs_status_idx;


ALTER TABLE sdd_designs RENAME CONSTRAINT designs_hu_id_fkey         TO sdd_designs_issue_id_fkey;
ALTER TABLE sdd_designs RENAME CONSTRAINT designs_proposal_id_fkey   TO sdd_designs_proposal_id_fkey;













ALTER TABLE IF EXISTS tasks RENAME TO issue_tasks;

ALTER INDEX IF EXISTS tasks_pkey RENAME TO issue_tasks_pkey;

ALTER TABLE issue_tasks RENAME CONSTRAINT tasks_hu_id_fkey TO issue_tasks_issue_id_fkey;




ALTER TABLE IF EXISTS code_references RENAME TO issue_code_references;

ALTER INDEX IF EXISTS code_references_pkey                 RENAME TO issue_code_references_pkey;
ALTER INDEX IF EXISTS code_references_hu_id_idx            RENAME TO issue_code_references_issue_id_idx;
ALTER INDEX IF EXISTS code_references_hu_id_file_path_key  RENAME TO issue_code_references_issue_id_file_path_key;
ALTER INDEX IF EXISTS code_references_file_path_idx        RENAME TO issue_code_references_file_path_idx;
ALTER INDEX IF EXISTS code_references_status_idx           RENAME TO issue_code_references_status_idx;


ALTER TABLE issue_code_references RENAME CONSTRAINT code_references_hu_id_fkey          TO issue_code_references_issue_id_fkey;





ALTER TABLE IF EXISTS intake_payloads RENAME TO issue_intake_payloads;

ALTER INDEX IF EXISTS intake_payloads_pkey          RENAME TO issue_intake_payloads_pkey;
ALTER INDEX IF EXISTS intake_payloads_reviewer_idx  RENAME TO issue_intake_payloads_reviewer_idx;
ALTER INDEX IF EXISTS intake_payloads_source_idx    RENAME TO issue_intake_payloads_source_idx;
ALTER INDEX IF EXISTS intake_payloads_status_idx    RENAME TO issue_intake_payloads_status_idx;

ALTER TABLE issue_intake_payloads RENAME CONSTRAINT intake_payloads_source_check             TO issue_intake_payloads_source_check;
ALTER TABLE issue_intake_payloads RENAME CONSTRAINT intake_payloads_status_check             TO issue_intake_payloads_status_check;
ALTER TABLE issue_intake_payloads RENAME CONSTRAINT intake_payloads_committed_hu_id_fkey     TO issue_intake_payloads_committed_issue_id_fkey;
ALTER TABLE issue_intake_payloads RENAME CONSTRAINT intake_payloads_committed_req_id_fkey    TO issue_intake_payloads_committed_req_id_fkey;
ALTER TABLE issue_intake_payloads RENAME CONSTRAINT intake_payloads_reviewer_id_fkey         TO issue_intake_payloads_reviewer_id_fkey;

DROP TRIGGER IF EXISTS set_updated_at_intake_payloads ON issue_intake_payloads;
CREATE TRIGGER set_updated_at_issue_intake_payloads
  BEFORE UPDATE ON issue_intake_payloads
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();






ALTER TABLE IF EXISTS verifications RENAME TO tdd_verifications;

ALTER INDEX IF EXISTS verifications_pkey                    RENAME TO tdd_verifications_pkey;
ALTER INDEX IF EXISTS verifications_org_project_status_idx  RENAME TO tdd_verifications_project_status_idx;
ALTER INDEX IF EXISTS verifications_org_session_idx         RENAME TO tdd_verifications_session_idx;
ALTER INDEX IF EXISTS verifications_pending_idx             RENAME TO tdd_verifications_pending_idx;

ALTER TABLE tdd_verifications RENAME CONSTRAINT verifications_kind_check        TO tdd_verifications_kind_check;
ALTER TABLE tdd_verifications RENAME CONSTRAINT verifications_status_check      TO tdd_verifications_status_check;
ALTER TABLE tdd_verifications RENAME CONSTRAINT verifications_project_id_fkey   TO tdd_verifications_project_id_fkey;

ALTER TABLE tdd_verifications RENAME CONSTRAINT verifications_user_id_fkey      TO tdd_verifications_user_id_fkey;


ALTER TABLE tdd_verifications NO FORCE ROW LEVEL SECURITY;





ALTER TABLE IF EXISTS verification_results RENAME TO tdd_verification_results;

ALTER INDEX IF EXISTS verification_results_pkey RENAME TO tdd_verification_results_pkey;

ALTER TABLE tdd_verification_results RENAME CONSTRAINT verification_results_task_id_fkey TO tdd_verification_results_task_id_fkey;







ALTER TABLE IF EXISTS sabotage_records RENAME TO tdd_sabotage_records;

ALTER INDEX IF EXISTS sabotage_records_pkey       RENAME TO tdd_sabotage_records_pkey;
ALTER INDEX IF EXISTS sabotage_records_status_idx RENAME TO tdd_sabotage_records_status_idx;
ALTER INDEX IF EXISTS sabotage_task_id_idx        RENAME TO tdd_sabotage_records_task_id_idx;

ALTER TABLE tdd_sabotage_records RENAME CONSTRAINT sabotage_records_task_id_fkey TO tdd_sabotage_records_task_id_fkey;




DO $grants$
DECLARE
  tbl text;
BEGIN
  FOR tbl IN SELECT unnest(ARRAY[
    'sdd_requirements', 'sdd_proposals', 'sdd_designs',
    'issue_tasks', 'issue_code_references', 'issue_intake_payloads',
    'tdd_verifications', 'tdd_verification_results', 'tdd_sabotage_records'
  ]) LOOP
    BEGIN
      EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON public.%s TO app_user', tbl);
    EXCEPTION WHEN OTHERS THEN NULL;
    END;
    BEGIN
      EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON public.%s TO app_admin', tbl);
    EXCEPTION WHEN OTHERS THEN NULL;
    END;
    BEGIN
      EXECUTE format('GRANT SELECT ON public.%s TO app_readonly', tbl);
    EXCEPTION WHEN OTHERS THEN NULL;
    END;
  END LOOP;
END
$grants$;

COMMIT;
