




DO $$
DECLARE
    r RECORD;
    add_count INT := 0;
BEGIN



    FOR r IN (
        SELECT unnest(ARRAY[
            'users', 'api_keys', 'projects', 'observations', 'sessions',
            'prompts', 'knowledge_docs', 'knowledge_chunks', 'skills',
            'agents', 'flows', 'flow_runs', 'agent_runs', 'crons',
            'webhooks', 'webhook_deliveries', 'audit_log', 'secrets',
            'cost_logs', 'project_templates', 'project_links',
            'project_merges', 'otp_codes', 'activity_log',
            'auth_otp_codes', 'org_cost_alert_thresholds',
            'org_flow_config', 'usage_counters', 'org_enrollment_tokens',
            'idempotency_keys', 'outbound_webhook_subscriptions',
            'outbound_webhook_deliveries', 'usage_alerts',
            'usage_alert_fires', 'mcp_servers', 'mcp_server_tools',
            'notification_deliveries', 'roles', 'user_roles',
            'auth_invitations', 'auth_events', 'hu_drafts',
            'intake_payloads', 'external_providers', 'external_sync_state',
            'external_sync_events', 'event_log', 'llm_semantic_cache',
            'agent_templates', 'selfhosted_runners',
            'selfhosted_tasks', 'imported_workflow_files',
            'dead_letter_queue', 'skill_executions', 'budgets',
            'mcp_providers', 'clients', 'captured_prompts',
            'project_repositories', 'project_policies',
            'project_policy_versions', 'verifications',
            'project_tickets', 'project_ticket_comments',
            'project_ticket_status_history', 'project_index_runs'
        ]) AS table_name
    ) LOOP
        -- La lista es un snapshot con nombres que migraciones posteriores
        -- renombraron/dropearon (auth_api_keys->api_keys, observations->
        -- knowledge_observations, etc). En el down chain esos renames ya se
        -- revirtieron/aplicaron, asi que muchas tablas del listado no existen
        -- con ese nombre. ALTER TABLE IF EXISTS salta las ausentes sin romper.
        EXECUTE format('ALTER TABLE IF EXISTS %I ADD COLUMN IF NOT EXISTS organization_id UUID',
                       r.table_name);
        add_count := add_count + 1;
    END LOOP;
    RAISE NOTICE 'total columns added: %', add_count;
END $$;
