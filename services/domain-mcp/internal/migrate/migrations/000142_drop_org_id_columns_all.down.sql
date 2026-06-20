-- Revertir: re-crear organization_id como UUID nullable en TODAS las tablas
-- que la tenían. NO restaura los valores (esos se perdieron en el up). En
-- roundtrip (DB fresca) no hay filas, así que el reverse es seguro. En DB
-- con datos, queda una columna vacía — los joins/filtros por organization_id
-- dejarán de matchear nada.
DO $$
DECLARE
    r RECORD;
    add_count INT := 0;
BEGIN
    -- Para revertir limpiamente, restauramos las columnas en las 54 tablas
    -- históricas. Si una tabla ya fue dropeada por otra migración, el
    -- ADD COLUMN IF NOT EXISTS es no-op (no falla).
    FOR r IN (
        SELECT unnest(ARRAY[
            'users', 'api_keys', 'projects', 'observations', 'sessions',
            'prompts', 'knowledge_docs', 'knowledge_chunks', 'skills',
            'agents', 'flows', 'flow_runs', 'agent_runs', 'crons',
            'webhooks', 'webhook_deliveries', 'audit_log', 'secrets',
            'cost_logs', 'project_templates', 'project_links',
            'project_merges', 'otp_codes', 'activity_log',
            'cost_alerts_sent', 'org_cost_alert_thresholds',
            'org_flow_config', 'usage_counters', 'org_enrollment_tokens',
            'idempotency_keys', 'outbound_webhook_subscriptions',
            'outbound_webhook_deliveries', 'usage_alerts',
            'usage_alert_fires', 'mcp_servers', 'mcp_server_tools',
            'notification_deliveries', 'roles', 'user_roles',
            'auth_sessions', 'auth_events', 'hu_drafts',
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
        EXECUTE format('ALTER TABLE %I ADD COLUMN IF NOT EXISTS organization_id UUID',
                       r.table_name);
        add_count := add_count + 1;
    END LOOP;
    RAISE NOTICE 'total columns added: %', add_count;
END $$;
