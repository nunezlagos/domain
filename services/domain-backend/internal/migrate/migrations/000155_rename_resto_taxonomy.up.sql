-- migration: rename_resto_taxonomy
-- author: mnunez@saargo.com
-- issue: REQ-42.9 (schema naming taxonomy — cierre de renames)
-- description: rename del RESTO de tablas con action=rename que no entraron
--   en las HU 42.5-42.8, aplicando el prefijo de su grupo funcional segun
--   la taxonomia. Single-org, DB casi vacia (9 tablas con 0 rows), riesgo
--   de datos NULO. Rename atomico directo estilo migration 000146 (NO
--   expand/contract). Las FK se preservan por OID (el ALTER TABLE RENAME
--   no rompe referencias entrantes ni salientes).
--
--   DE-SCOPE (ya cubiertos por HU previas, NO van aqui):
--   - org_enrollment_tokens -> enrollment_tokens  ........ HU 42.7 (000153)
--   - verifications          -> tdd_verifications  ....... HU 42.5 (000151)
--   - verification_results   -> tdd_verification_results . HU 42.5 (000151)
--
--   Renames de esta HU (9 tablas):
--   1. captured_prompts               -> prompt_captured
--   2. clients                        -> project_clients
--   3. imported_workflow_files        -> project_imported_workflow_files
--   4. observations                   -> knowledge_observations
--   5. outbound_webhook_subscriptions -> webhook_outbound_subscriptions
--   6. outbound_webhook_deliveries    -> webhook_outbound_deliveries
--   7. selfhosted_runners             -> runner_selfhosted
--   8. selfhosted_tasks               -> runner_selfhosted_tasks
--   9. activity_log                   -> audit_activity_log
--
--   Trampas honradas:
--   - pkey y UNIQUE con indice homonimo: SOLO ALTER INDEX (el RENAME
--     CONSTRAINT adicional fallaria por duplicado; comparten objeto).
--   - pares con FK interna (webhook subs<->deliveries, runner runners<->tasks)
--     renombrados en la MISMA transaccion.
--   - ninguna tabla tiene sequence (PK UUID) ni RLS policy activa
--     (relrowsecurity=f; FORCE sin ENABLE es inerte, no hay policy).
--
--   NO toca la tabla `sessions` (marcada DROP en otra HU); la FK
--   captured_prompts.session_id se preserva por OID. Coordinar orden con
--   el drop de sessions (limpieza de session_id).
--
--   down: RENAME reverso exacto (atomico).
-- breaking: false (rename de naming interno; API publica no afectada)
-- estimated_duration: <1s

BEGIN;

-- 1) captured_prompts -> prompt_captured
ALTER TABLE captured_prompts RENAME TO prompt_captured;
ALTER INDEX captured_prompts_pkey       RENAME TO prompt_captured_pkey;
ALTER INDEX captured_prompts_status_idx RENAME TO prompt_captured_status_idx;
ALTER INDEX captured_prompts_tsv_idx    RENAME TO prompt_captured_tsv_idx;
ALTER TABLE prompt_captured RENAME CONSTRAINT captured_prompts_project_id_fkey TO prompt_captured_project_id_fkey;
-- NOTA: session_id (FK + columna) fue dropeada en 000149 (sessions legacy) — no hay constraint que renombrar.
ALTER TABLE prompt_captured RENAME CONSTRAINT captured_prompts_user_id_fkey    TO prompt_captured_user_id_fkey;

-- 2) clients -> project_clients (2 FK entrantes preservadas por OID)
ALTER TABLE clients RENAME TO project_clients;
ALTER INDEX clients_pkey       RENAME TO project_clients_pkey;
ALTER INDEX clients_status_idx RENAME TO project_clients_status_idx;
ALTER TABLE project_clients RENAME CONSTRAINT clients_status_check TO project_clients_status_check;

-- 3) imported_workflow_files -> project_imported_workflow_files
ALTER TABLE imported_workflow_files RENAME TO project_imported_workflow_files;
ALTER INDEX imported_workflow_files_pkey        RENAME TO project_imported_workflow_files_pkey;
ALTER INDEX imported_workflow_files_project_idx RENAME TO project_imported_workflow_files_project_idx;
ALTER INDEX imported_workflow_files_status_idx  RENAME TO project_imported_workflow_files_status_idx;
ALTER INDEX imported_workflow_files_tool_idx    RENAME TO project_imported_workflow_files_tool_idx;
ALTER INDEX imported_workflow_files_unique      RENAME TO project_imported_workflow_files_unique;
ALTER TABLE project_imported_workflow_files RENAME CONSTRAINT imported_workflow_files_project_id_fkey   TO project_imported_workflow_files_project_id_fkey;
ALTER TABLE project_imported_workflow_files RENAME CONSTRAINT imported_workflow_files_source_tool_check TO project_imported_workflow_files_source_tool_check;
ALTER TABLE project_imported_workflow_files RENAME CONSTRAINT imported_workflow_files_status_check      TO project_imported_workflow_files_status_check;

-- 4) observations -> knowledge_observations (ivfflat + GIN conservan metodo)
ALTER TABLE observations RENAME TO knowledge_observations;
ALTER INDEX observations_pkey                RENAME TO knowledge_observations_pkey;
ALTER INDEX observations_status_idx          RENAME TO knowledge_observations_status_idx;
ALTER INDEX observations_content_tsv_idx     RENAME TO knowledge_observations_content_tsv_idx;
ALTER INDEX observations_dedup_hash_uniq     RENAME TO knowledge_observations_dedup_hash_uniq;
ALTER INDEX observations_embedding_idx       RENAME TO knowledge_observations_embedding_idx;
ALTER INDEX observations_project_created_idx RENAME TO knowledge_observations_project_created_idx;
ALTER INDEX observations_tags_idx            RENAME TO knowledge_observations_tags_idx;
ALTER TABLE knowledge_observations RENAME CONSTRAINT observations_created_by_fkey TO knowledge_observations_created_by_fkey;
ALTER TABLE knowledge_observations RENAME CONSTRAINT observations_project_id_fkey TO knowledge_observations_project_id_fkey;

-- 5) outbound_webhook_subscriptions -> webhook_outbound_subscriptions (par con deliveries)
ALTER TABLE outbound_webhook_subscriptions RENAME TO webhook_outbound_subscriptions;
ALTER INDEX outbound_webhook_subscriptions_pkey       RENAME TO webhook_outbound_subscriptions_pkey;
ALTER INDEX outbound_webhook_subscriptions_status_idx RENAME TO webhook_outbound_subscriptions_status_idx;
ALTER INDEX outbound_webhook_subscriptions_events_gin RENAME TO webhook_outbound_subscriptions_events_gin;
ALTER TABLE webhook_outbound_subscriptions RENAME CONSTRAINT outbound_webhook_subscriptions_url_check TO webhook_outbound_subscriptions_url_check;

-- 6) outbound_webhook_deliveries -> webhook_outbound_deliveries
ALTER TABLE outbound_webhook_deliveries RENAME TO webhook_outbound_deliveries;
ALTER INDEX outbound_webhook_deliveries_pkey        RENAME TO webhook_outbound_deliveries_pkey;
ALTER INDEX outbound_webhook_deliveries_status_idx  RENAME TO webhook_outbound_deliveries_status_idx;
ALTER INDEX outbound_webhook_deliveries_pending_idx RENAME TO webhook_outbound_deliveries_pending_idx;
ALTER INDEX outbound_webhook_deliveries_sub_idx     RENAME TO webhook_outbound_deliveries_sub_idx;
ALTER TABLE webhook_outbound_deliveries RENAME CONSTRAINT outbound_webhook_deliveries_status_check         TO webhook_outbound_deliveries_status_check;
ALTER TABLE webhook_outbound_deliveries RENAME CONSTRAINT outbound_webhook_deliveries_subscription_id_fkey TO webhook_outbound_deliveries_subscription_id_fkey;

-- 7) selfhosted_runners -> runner_selfhosted (par con tasks)
ALTER TABLE selfhosted_runners RENAME TO runner_selfhosted;
ALTER INDEX selfhosted_runners_pkey          RENAME TO runner_selfhosted_pkey;
ALTER INDEX selfhosted_runners_status_idx    RENAME TO runner_selfhosted_status_idx;
ALTER INDEX selfhosted_runners_heartbeat_idx RENAME TO runner_selfhosted_heartbeat_idx;

-- 8) selfhosted_tasks -> runner_selfhosted_tasks
ALTER TABLE selfhosted_tasks RENAME TO runner_selfhosted_tasks;
ALTER INDEX selfhosted_tasks_pkey        RENAME TO runner_selfhosted_tasks_pkey;
ALTER INDEX selfhosted_tasks_status_idx  RENAME TO runner_selfhosted_tasks_status_idx;
ALTER INDEX selfhosted_tasks_claimed_idx RENAME TO runner_selfhosted_tasks_claimed_idx;
ALTER TABLE runner_selfhosted_tasks RENAME CONSTRAINT selfhosted_tasks_status_check    TO runner_selfhosted_tasks_status_check;
ALTER TABLE runner_selfhosted_tasks RENAME CONSTRAINT selfhosted_tasks_claimed_by_fkey TO runner_selfhosted_tasks_claimed_by_fkey;

-- 9) activity_log -> audit_activity_log (FORCE RLS inerte, sin policy)
ALTER TABLE activity_log RENAME TO audit_activity_log;
ALTER INDEX activity_log_pkey        RENAME TO audit_activity_log_pkey;
ALTER INDEX activity_log_status_idx  RENAME TO audit_activity_log_status_idx;
ALTER INDEX activity_log_actor_idx   RENAME TO audit_activity_log_actor_idx;
ALTER INDEX activity_log_entity_idx  RENAME TO audit_activity_log_entity_idx;
ALTER INDEX activity_log_project_idx RENAME TO audit_activity_log_project_idx;
ALTER TABLE audit_activity_log RENAME CONSTRAINT activity_log_actor_id_fkey    TO audit_activity_log_actor_id_fkey;
ALTER TABLE audit_activity_log RENAME CONSTRAINT activity_log_project_id_fkey  TO audit_activity_log_project_id_fkey;
ALTER TABLE audit_activity_log RENAME CONSTRAINT activity_log_visibility_check TO audit_activity_log_visibility_check;

COMMIT;
