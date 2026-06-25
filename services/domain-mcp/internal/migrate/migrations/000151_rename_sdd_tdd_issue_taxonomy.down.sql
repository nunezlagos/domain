











BEGIN;




ALTER TABLE IF EXISTS tdd_sabotage_records RENAME TO sabotage_records;

ALTER INDEX IF EXISTS tdd_sabotage_records_pkey         RENAME TO sabotage_records_pkey;
ALTER INDEX IF EXISTS tdd_sabotage_records_status_idx   RENAME TO sabotage_records_status_idx;
ALTER INDEX IF EXISTS tdd_sabotage_records_task_id_idx  RENAME TO sabotage_task_id_idx;

ALTER TABLE sabotage_records RENAME CONSTRAINT tdd_sabotage_records_task_id_fkey TO sabotage_records_task_id_fkey;




ALTER TABLE IF EXISTS tdd_verification_results RENAME TO verification_results;

ALTER INDEX IF EXISTS tdd_verification_results_pkey RENAME TO verification_results_pkey;

ALTER TABLE verification_results RENAME CONSTRAINT tdd_verification_results_task_id_fkey TO verification_results_task_id_fkey;




ALTER TABLE IF EXISTS tdd_verifications RENAME TO verifications;

ALTER INDEX IF EXISTS tdd_verifications_pkey                RENAME TO verifications_pkey;
ALTER INDEX IF EXISTS tdd_verifications_project_status_idx  RENAME TO verifications_org_project_status_idx;
ALTER INDEX IF EXISTS tdd_verifications_session_idx         RENAME TO verifications_org_session_idx;
ALTER INDEX IF EXISTS tdd_verifications_pending_idx         RENAME TO verifications_pending_idx;

ALTER TABLE verifications RENAME CONSTRAINT tdd_verifications_kind_check       TO verifications_kind_check;
ALTER TABLE verifications RENAME CONSTRAINT tdd_verifications_status_check     TO verifications_status_check;
ALTER TABLE verifications RENAME CONSTRAINT tdd_verifications_project_id_fkey  TO verifications_project_id_fkey;

ALTER TABLE verifications RENAME CONSTRAINT tdd_verifications_user_id_fkey     TO verifications_user_id_fkey;




DROP TRIGGER IF EXISTS set_updated_at_issue_intake_payloads ON issue_intake_payloads;

ALTER TABLE IF EXISTS issue_intake_payloads RENAME TO intake_payloads;

ALTER INDEX IF EXISTS issue_intake_payloads_pkey          RENAME TO intake_payloads_pkey;
ALTER INDEX IF EXISTS issue_intake_payloads_reviewer_idx  RENAME TO intake_payloads_reviewer_idx;
ALTER INDEX IF EXISTS issue_intake_payloads_source_idx    RENAME TO intake_payloads_source_idx;
ALTER INDEX IF EXISTS issue_intake_payloads_status_idx    RENAME TO intake_payloads_status_idx;

ALTER TABLE intake_payloads RENAME CONSTRAINT issue_intake_payloads_source_check             TO intake_payloads_source_check;
ALTER TABLE intake_payloads RENAME CONSTRAINT issue_intake_payloads_status_check             TO intake_payloads_status_check;
ALTER TABLE intake_payloads RENAME CONSTRAINT issue_intake_payloads_committed_issue_id_fkey  TO intake_payloads_committed_hu_id_fkey;
ALTER TABLE intake_payloads RENAME CONSTRAINT issue_intake_payloads_committed_req_id_fkey    TO intake_payloads_committed_req_id_fkey;
ALTER TABLE intake_payloads RENAME CONSTRAINT issue_intake_payloads_reviewer_id_fkey         TO intake_payloads_reviewer_id_fkey;

CREATE TRIGGER set_updated_at_intake_payloads
  BEFORE UPDATE ON intake_payloads
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();




ALTER TABLE IF EXISTS issue_code_references RENAME TO code_references;

ALTER INDEX IF EXISTS issue_code_references_pkey                 RENAME TO code_references_pkey;
ALTER INDEX IF EXISTS issue_code_references_issue_id_idx         RENAME TO code_references_hu_id_idx;
ALTER INDEX IF EXISTS issue_code_references_issue_id_file_path_key RENAME TO code_references_hu_id_file_path_key;
ALTER INDEX IF EXISTS issue_code_references_file_path_idx        RENAME TO code_references_file_path_idx;
ALTER INDEX IF EXISTS issue_code_references_status_idx           RENAME TO code_references_status_idx;


ALTER TABLE code_references RENAME CONSTRAINT issue_code_references_issue_id_fkey          TO code_references_hu_id_fkey;




ALTER TABLE IF EXISTS issue_tasks RENAME TO tasks;

ALTER INDEX IF EXISTS issue_tasks_pkey RENAME TO tasks_pkey;

ALTER TABLE tasks RENAME CONSTRAINT issue_tasks_issue_id_fkey TO tasks_hu_id_fkey;










ALTER TABLE IF EXISTS sdd_designs RENAME TO designs;

ALTER INDEX IF EXISTS sdd_designs_pkey               RENAME TO designs_pkey;
ALTER INDEX IF EXISTS sdd_designs_issue_id_version_key RENAME TO designs_hu_id_version_key;
ALTER INDEX IF EXISTS sdd_designs_status_idx         RENAME TO designs_status_idx;


ALTER TABLE designs RENAME CONSTRAINT sdd_designs_issue_id_fkey        TO designs_hu_id_fkey;
ALTER TABLE designs RENAME CONSTRAINT sdd_designs_proposal_id_fkey     TO designs_proposal_id_fkey;




ALTER TABLE IF EXISTS sdd_proposals RENAME TO proposals;

ALTER INDEX IF EXISTS sdd_proposals_pkey                 RENAME TO proposals_pkey;
ALTER INDEX IF EXISTS sdd_proposals_issue_id_version_key RENAME TO proposals_hu_id_version_key;
ALTER INDEX IF EXISTS sdd_proposals_status_idx           RENAME TO proposals_status_idx;


ALTER TABLE proposals RENAME CONSTRAINT sdd_proposals_issue_id_fkey        TO proposals_hu_id_fkey;




ALTER TABLE IF EXISTS sdd_requirements RENAME TO requirements;

ALTER INDEX IF EXISTS sdd_requirements_pkey          RENAME TO requirements_pkey;
ALTER INDEX IF EXISTS sdd_requirements_parent_id_idx RENAME TO requirements_parent_id_idx;
ALTER INDEX IF EXISTS sdd_requirements_priority_idx  RENAME TO requirements_priority_idx;
ALTER INDEX IF EXISTS sdd_requirements_slug_idx      RENAME TO requirements_slug_idx;
ALTER INDEX IF EXISTS sdd_requirements_status_idx    RENAME TO requirements_status_idx;

ALTER TABLE requirements RENAME CONSTRAINT sdd_requirements_parent_id_fkey TO requirements_parent_id_fkey;

COMMIT;
