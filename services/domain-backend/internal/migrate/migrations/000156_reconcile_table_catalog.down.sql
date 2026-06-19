-- migration: reconcile_table_catalog_with_renames (down)
-- author: mnunez@saargo.com
-- issue: REQ-42.10
-- description: revierte la reconciliación del catálogo a los nombres previos
--   (pre-rename) y elimina las entradas nuevas (agent_run_prompts, table_catalog).
--   Idempotente. NOTA: gherkin_scenarios lo maneja el down de 000152.
-- breaking: false
-- estimated_duration: <1s

BEGIN;

DELETE FROM table_catalog WHERE table_name IN ('agent_run_prompts', 'table_catalog');

-- auth_
UPDATE table_catalog SET table_name = 'otp_codes'             WHERE table_name = 'auth_otp_codes';
UPDATE table_catalog SET table_name = 'api_keys'              WHERE table_name = 'auth_api_keys';
UPDATE table_catalog SET table_name = 'secrets'              WHERE table_name = 'auth_secrets';
UPDATE table_catalog SET table_name = 'invitations'          WHERE table_name = 'auth_invitations';
UPDATE table_catalog SET table_name = 'org_enrollment_tokens' WHERE table_name = 'enrollment_tokens';

-- prompt_
UPDATE table_catalog SET table_name = 'captured_prompts' WHERE table_name = 'prompt_captured';

-- project_
UPDATE table_catalog SET table_name = 'clients'                 WHERE table_name = 'project_clients';
UPDATE table_catalog SET table_name = 'imported_workflow_files' WHERE table_name = 'project_imported_workflow_files';

-- sdd_
UPDATE table_catalog SET table_name = 'requirements' WHERE table_name = 'sdd_requirements';
UPDATE table_catalog SET table_name = 'proposals'    WHERE table_name = 'sdd_proposals';
UPDATE table_catalog SET table_name = 'designs'      WHERE table_name = 'sdd_designs';

-- issue_
UPDATE table_catalog SET table_name = 'tasks'           WHERE table_name = 'issue_tasks';
UPDATE table_catalog SET table_name = 'code_references' WHERE table_name = 'issue_code_references';
UPDATE table_catalog SET table_name = 'intake_payloads' WHERE table_name = 'issue_intake_payloads';

-- tdd_
UPDATE table_catalog SET table_name = 'verifications'        WHERE table_name = 'tdd_verifications';
UPDATE table_catalog SET table_name = 'verification_results' WHERE table_name = 'tdd_verification_results';
UPDATE table_catalog SET table_name = 'sabotage_records'     WHERE table_name = 'tdd_sabotage_records';

-- knowledge_
UPDATE table_catalog SET table_name = 'observations' WHERE table_name = 'knowledge_observations';

-- webhook_
UPDATE table_catalog SET table_name = 'outbound_webhook_subscriptions' WHERE table_name = 'webhook_outbound_subscriptions';
UPDATE table_catalog SET table_name = 'outbound_webhook_deliveries'    WHERE table_name = 'webhook_outbound_deliveries';

-- runner_
UPDATE table_catalog SET table_name = 'selfhosted_runners' WHERE table_name = 'runner_selfhosted';
UPDATE table_catalog SET table_name = 'selfhosted_tasks'   WHERE table_name = 'runner_selfhosted_tasks';

-- audit_
UPDATE table_catalog SET table_name = 'activity_log' WHERE table_name = 'audit_activity_log';

COMMIT;
