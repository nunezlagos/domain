-- migration: reconcile_table_catalog_with_renames
-- author: mnunez@saargo.com
-- issue: REQ-42.10 (schema naming taxonomy — sincroniza el catálogo funcional)
-- description: las HUs 42.5-42.9 renombraron tablas pero NO actualizaron
--   table_catalog (solo 000152/gherkin lo hizo). Este migration reconcilia el
--   catálogo con los nombres ACTUALES para que /admin/database agrupe por
--   FUNCIONALIDAD leyendo table_catalog (grupo/label), no por prefijo crudo.
--   Idempotente: UPDATE ... WHERE <nombre_viejo> (no-op si ya migrado) +
--   INSERT ... ON CONFLICT. Agrega agent_run_prompts (HU 42.4) y la propia
--   table_catalog (grupo internal, oculta en la UI).
-- breaking: false (solo datos de catálogo; no toca tablas reales)
-- estimated_duration: <1s

BEGIN;

-- auth_
UPDATE table_catalog SET table_name = 'auth_otp_codes'    WHERE table_name = 'otp_codes';
UPDATE table_catalog SET table_name = 'auth_api_keys'     WHERE table_name = 'api_keys';
UPDATE table_catalog SET table_name = 'auth_secrets'      WHERE table_name = 'secrets';
UPDATE table_catalog SET table_name = 'auth_invitations'  WHERE table_name = 'invitations';
UPDATE table_catalog SET table_name = 'enrollment_tokens' WHERE table_name = 'org_enrollment_tokens';

-- prompt_
UPDATE table_catalog SET table_name = 'prompt_captured' WHERE table_name = 'captured_prompts';

-- project_
UPDATE table_catalog SET table_name = 'project_clients'                 WHERE table_name = 'clients';
UPDATE table_catalog SET table_name = 'project_imported_workflow_files' WHERE table_name = 'imported_workflow_files';

-- sdd_
UPDATE table_catalog SET table_name = 'sdd_requirements' WHERE table_name = 'requirements';
UPDATE table_catalog SET table_name = 'sdd_proposals'    WHERE table_name = 'proposals';
UPDATE table_catalog SET table_name = 'sdd_designs'      WHERE table_name = 'designs';

-- issue_  (gherkin_scenarios ya lo actualizó 000152)
UPDATE table_catalog SET table_name = 'issue_tasks'           WHERE table_name = 'tasks';
UPDATE table_catalog SET table_name = 'issue_code_references' WHERE table_name = 'code_references';
UPDATE table_catalog SET table_name = 'issue_intake_payloads' WHERE table_name = 'intake_payloads';

-- tdd_
UPDATE table_catalog SET table_name = 'tdd_verifications'        WHERE table_name = 'verifications';
UPDATE table_catalog SET table_name = 'tdd_verification_results' WHERE table_name = 'verification_results';
UPDATE table_catalog SET table_name = 'tdd_sabotage_records'     WHERE table_name = 'sabotage_records';

-- knowledge_
UPDATE table_catalog SET table_name = 'knowledge_observations' WHERE table_name = 'observations';

-- webhook_
UPDATE table_catalog SET table_name = 'webhook_outbound_subscriptions' WHERE table_name = 'outbound_webhook_subscriptions';
UPDATE table_catalog SET table_name = 'webhook_outbound_deliveries'    WHERE table_name = 'outbound_webhook_deliveries';

-- runner_
UPDATE table_catalog SET table_name = 'runner_selfhosted'       WHERE table_name = 'selfhosted_runners';
UPDATE table_catalog SET table_name = 'runner_selfhosted_tasks' WHERE table_name = 'selfhosted_tasks';

-- audit_
UPDATE table_catalog SET table_name = 'audit_activity_log' WHERE table_name = 'activity_log';

-- tablas nuevas (agent_run_prompts = HU 42.4; table_catalog = la propia, oculta)
INSERT INTO table_catalog (table_name, grupo, label, sort_order) VALUES
  ('agent_run_prompts', 'agent',    'Agentes',          306),
  ('table_catalog',     'internal', 'Interno (oculto)', 9902)
ON CONFLICT (table_name) DO UPDATE
  SET grupo = EXCLUDED.grupo, label = EXCLUDED.label, sort_order = EXCLUDED.sort_order;

COMMIT;
