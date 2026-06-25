









































BEGIN;


ALTER TABLE captured_prompts RENAME TO prompt_captured;
ALTER INDEX captured_prompts_pkey       RENAME TO prompt_captured_pkey;
ALTER INDEX captured_prompts_status_idx RENAME TO prompt_captured_status_idx;
ALTER INDEX captured_prompts_tsv_idx    RENAME TO prompt_captured_tsv_idx;
ALTER TABLE prompt_captured RENAME CONSTRAINT captured_prompts_project_id_fkey TO prompt_captured_project_id_fkey;

ALTER TABLE prompt_captured RENAME CONSTRAINT captured_prompts_user_id_fkey    TO prompt_captured_user_id_fkey;


ALTER TABLE clients RENAME TO project_clients;
ALTER INDEX clients_pkey       RENAME TO project_clients_pkey;
ALTER INDEX clients_status_idx RENAME TO project_clients_status_idx;
ALTER TABLE project_clients RENAME CONSTRAINT clients_status_check TO project_clients_status_check;


ALTER TABLE imported_workflow_files RENAME TO project_imported_workflow_files;
ALTER INDEX imported_workflow_files_pkey        RENAME TO project_imported_workflow_files_pkey;
ALTER INDEX imported_workflow_files_project_idx RENAME TO project_imported_workflow_files_project_idx;
ALTER INDEX imported_workflow_files_status_idx  RENAME TO project_imported_workflow_files_status_idx;
ALTER INDEX imported_workflow_files_tool_idx    RENAME TO project_imported_workflow_files_tool_idx;
ALTER INDEX imported_workflow_files_unique      RENAME TO project_imported_workflow_files_unique;
ALTER TABLE project_imported_workflow_files RENAME CONSTRAINT imported_workflow_files_project_id_fkey   TO project_imported_workflow_files_project_id_fkey;
ALTER TABLE project_imported_workflow_files RENAME CONSTRAINT imported_workflow_files_source_tool_check TO project_imported_workflow_files_source_tool_check;
ALTER TABLE project_imported_workflow_files RENAME CONSTRAINT imported_workflow_files_status_check      TO project_imported_workflow_files_status_check;


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


ALTER TABLE outbound_webhook_subscriptions RENAME TO webhook_outbound_subscriptions;
ALTER INDEX outbound_webhook_subscriptions_pkey       RENAME TO webhook_outbound_subscriptions_pkey;
ALTER INDEX outbound_webhook_subscriptions_status_idx RENAME TO webhook_outbound_subscriptions_status_idx;
ALTER INDEX outbound_webhook_subscriptions_events_gin RENAME TO webhook_outbound_subscriptions_events_gin;
ALTER TABLE webhook_outbound_subscriptions RENAME CONSTRAINT outbound_webhook_subscriptions_url_check TO webhook_outbound_subscriptions_url_check;


ALTER TABLE outbound_webhook_deliveries RENAME TO webhook_outbound_deliveries;
ALTER INDEX outbound_webhook_deliveries_pkey        RENAME TO webhook_outbound_deliveries_pkey;
ALTER INDEX outbound_webhook_deliveries_status_idx  RENAME TO webhook_outbound_deliveries_status_idx;
ALTER INDEX outbound_webhook_deliveries_pending_idx RENAME TO webhook_outbound_deliveries_pending_idx;
ALTER INDEX outbound_webhook_deliveries_sub_idx     RENAME TO webhook_outbound_deliveries_sub_idx;
ALTER TABLE webhook_outbound_deliveries RENAME CONSTRAINT outbound_webhook_deliveries_status_check         TO webhook_outbound_deliveries_status_check;
ALTER TABLE webhook_outbound_deliveries RENAME CONSTRAINT outbound_webhook_deliveries_subscription_id_fkey TO webhook_outbound_deliveries_subscription_id_fkey;


ALTER TABLE selfhosted_runners RENAME TO runner_selfhosted;
ALTER INDEX selfhosted_runners_pkey          RENAME TO runner_selfhosted_pkey;
ALTER INDEX selfhosted_runners_status_idx    RENAME TO runner_selfhosted_status_idx;
ALTER INDEX selfhosted_runners_heartbeat_idx RENAME TO runner_selfhosted_heartbeat_idx;


ALTER TABLE selfhosted_tasks RENAME TO runner_selfhosted_tasks;
ALTER INDEX selfhosted_tasks_pkey        RENAME TO runner_selfhosted_tasks_pkey;
ALTER INDEX selfhosted_tasks_status_idx  RENAME TO runner_selfhosted_tasks_status_idx;
ALTER INDEX selfhosted_tasks_claimed_idx RENAME TO runner_selfhosted_tasks_claimed_idx;
ALTER TABLE runner_selfhosted_tasks RENAME CONSTRAINT selfhosted_tasks_status_check    TO runner_selfhosted_tasks_status_check;
ALTER TABLE runner_selfhosted_tasks RENAME CONSTRAINT selfhosted_tasks_claimed_by_fkey TO runner_selfhosted_tasks_claimed_by_fkey;


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
