










BEGIN;


ALTER TABLE audit_activity_log RENAME CONSTRAINT audit_activity_log_visibility_check TO activity_log_visibility_check;
ALTER TABLE audit_activity_log RENAME CONSTRAINT audit_activity_log_project_id_fkey  TO activity_log_project_id_fkey;
ALTER TABLE audit_activity_log RENAME CONSTRAINT audit_activity_log_actor_id_fkey    TO activity_log_actor_id_fkey;
ALTER INDEX audit_activity_log_project_idx RENAME TO activity_log_project_idx;
ALTER INDEX audit_activity_log_entity_idx  RENAME TO activity_log_entity_idx;
ALTER INDEX audit_activity_log_actor_idx   RENAME TO activity_log_actor_idx;
ALTER INDEX audit_activity_log_status_idx  RENAME TO activity_log_status_idx;
ALTER INDEX audit_activity_log_pkey        RENAME TO activity_log_pkey;
ALTER TABLE audit_activity_log RENAME TO activity_log;


ALTER TABLE runner_selfhosted_tasks RENAME CONSTRAINT runner_selfhosted_tasks_claimed_by_fkey TO selfhosted_tasks_claimed_by_fkey;
ALTER TABLE runner_selfhosted_tasks RENAME CONSTRAINT runner_selfhosted_tasks_status_check    TO selfhosted_tasks_status_check;
ALTER INDEX runner_selfhosted_tasks_claimed_idx RENAME TO selfhosted_tasks_claimed_idx;
ALTER INDEX runner_selfhosted_tasks_status_idx  RENAME TO selfhosted_tasks_status_idx;
ALTER INDEX runner_selfhosted_tasks_pkey        RENAME TO selfhosted_tasks_pkey;
ALTER TABLE runner_selfhosted_tasks RENAME TO selfhosted_tasks;


ALTER INDEX runner_selfhosted_heartbeat_idx RENAME TO selfhosted_runners_heartbeat_idx;
ALTER INDEX runner_selfhosted_status_idx    RENAME TO selfhosted_runners_status_idx;
ALTER INDEX runner_selfhosted_pkey          RENAME TO selfhosted_runners_pkey;
ALTER TABLE runner_selfhosted RENAME TO selfhosted_runners;


ALTER TABLE webhook_outbound_deliveries RENAME CONSTRAINT webhook_outbound_deliveries_subscription_id_fkey TO outbound_webhook_deliveries_subscription_id_fkey;
ALTER TABLE webhook_outbound_deliveries RENAME CONSTRAINT webhook_outbound_deliveries_status_check         TO outbound_webhook_deliveries_status_check;
ALTER INDEX webhook_outbound_deliveries_sub_idx     RENAME TO outbound_webhook_deliveries_sub_idx;
ALTER INDEX webhook_outbound_deliveries_pending_idx RENAME TO outbound_webhook_deliveries_pending_idx;
ALTER INDEX webhook_outbound_deliveries_status_idx  RENAME TO outbound_webhook_deliveries_status_idx;
ALTER INDEX webhook_outbound_deliveries_pkey        RENAME TO outbound_webhook_deliveries_pkey;
ALTER TABLE webhook_outbound_deliveries RENAME TO outbound_webhook_deliveries;


ALTER TABLE webhook_outbound_subscriptions RENAME CONSTRAINT webhook_outbound_subscriptions_url_check TO outbound_webhook_subscriptions_url_check;
ALTER INDEX webhook_outbound_subscriptions_events_gin RENAME TO outbound_webhook_subscriptions_events_gin;
ALTER INDEX webhook_outbound_subscriptions_status_idx RENAME TO outbound_webhook_subscriptions_status_idx;
ALTER INDEX webhook_outbound_subscriptions_pkey       RENAME TO outbound_webhook_subscriptions_pkey;
ALTER TABLE webhook_outbound_subscriptions RENAME TO outbound_webhook_subscriptions;


ALTER TABLE knowledge_observations RENAME CONSTRAINT knowledge_observations_project_id_fkey TO observations_project_id_fkey;
ALTER TABLE knowledge_observations RENAME CONSTRAINT knowledge_observations_created_by_fkey TO observations_created_by_fkey;
ALTER INDEX knowledge_observations_tags_idx            RENAME TO observations_tags_idx;
ALTER INDEX knowledge_observations_project_created_idx RENAME TO observations_project_created_idx;
ALTER INDEX knowledge_observations_embedding_idx       RENAME TO observations_embedding_idx;
ALTER INDEX knowledge_observations_dedup_hash_uniq     RENAME TO observations_dedup_hash_uniq;
ALTER INDEX knowledge_observations_content_tsv_idx     RENAME TO observations_content_tsv_idx;
ALTER INDEX knowledge_observations_status_idx          RENAME TO observations_status_idx;
ALTER INDEX knowledge_observations_pkey                RENAME TO observations_pkey;
ALTER TABLE knowledge_observations RENAME TO observations;


ALTER TABLE project_imported_workflow_files RENAME CONSTRAINT project_imported_workflow_files_status_check      TO imported_workflow_files_status_check;
ALTER TABLE project_imported_workflow_files RENAME CONSTRAINT project_imported_workflow_files_source_tool_check TO imported_workflow_files_source_tool_check;
ALTER TABLE project_imported_workflow_files RENAME CONSTRAINT project_imported_workflow_files_project_id_fkey   TO imported_workflow_files_project_id_fkey;
ALTER INDEX project_imported_workflow_files_unique      RENAME TO imported_workflow_files_unique;
ALTER INDEX project_imported_workflow_files_tool_idx    RENAME TO imported_workflow_files_tool_idx;
ALTER INDEX project_imported_workflow_files_status_idx  RENAME TO imported_workflow_files_status_idx;
ALTER INDEX project_imported_workflow_files_project_idx RENAME TO imported_workflow_files_project_idx;
ALTER INDEX project_imported_workflow_files_pkey        RENAME TO imported_workflow_files_pkey;
ALTER TABLE project_imported_workflow_files RENAME TO imported_workflow_files;


ALTER TABLE project_clients RENAME CONSTRAINT project_clients_status_check TO clients_status_check;
ALTER INDEX project_clients_status_idx RENAME TO clients_status_idx;
ALTER INDEX project_clients_pkey       RENAME TO clients_pkey;
ALTER TABLE project_clients RENAME TO clients;


ALTER TABLE prompt_captured RENAME CONSTRAINT prompt_captured_user_id_fkey    TO captured_prompts_user_id_fkey;

ALTER TABLE prompt_captured RENAME CONSTRAINT prompt_captured_project_id_fkey TO captured_prompts_project_id_fkey;
ALTER INDEX prompt_captured_tsv_idx    RENAME TO captured_prompts_tsv_idx;
ALTER INDEX prompt_captured_status_idx RENAME TO captured_prompts_status_idx;
ALTER INDEX prompt_captured_pkey       RENAME TO captured_prompts_pkey;
ALTER TABLE prompt_captured RENAME TO captured_prompts;

COMMIT;
