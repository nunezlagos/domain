-- migration: 000172_chat_ia_readonly_role
-- description: crea rol `domain_chat_reader` con permisos minimos para que
--   domain-admin (chat) NO pueda escribir sobre tablas del dominio.
--   Previene LLM08 (Excessive Agency) y limita el blast radius si el
--   bot se compromete.
-- breaking: no (no cambia permisos de app_user ni app_admin; solo crea rol nuevo).
-- Aplica: domain-mcp. domain-admin debe migrar a usar este rol.

-- ============================================================
-- 1) Crear rol readonly para el chat
-- ============================================================
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'domain_chat_reader') THEN
        CREATE ROLE domain_chat_reader NOLOGIN;
    END IF;
END
$$;

-- ============================================================
-- 2) Permisos: SELECT en todas las tablas del dominio (read-only)
-- ============================================================
GRANT USAGE ON SCHEMA public TO domain_chat_reader;

-- Solo lectura sobre las tablas del dominio (agents, skills, flows, etc).
-- No incluye las tablas de chat_* (esas tienen INSERT para el bot).
DO $$
DECLARE
    t TEXT;
    domain_tables TEXT[] := ARRAY[
        'agents', 'agent_templates', 'agent_versions', 'agent_runs',
        'skills', 'skill_versions', 'skill_executions',
        'flows', 'flow_versions', 'flow_runs', 'flow_run_steps',
        'prompts',
        'projects', 'project_templates', 'project_repositories', 'project_skills',
        'project_tickets', 'project_ticket_comments', 'project_policies',
        'users', 'roles', 'user_roles',
        'clients', 'client_contacts',
        'issues',
        'knowledge_docs', 'knowledge_chunks', 'knowledge_observations',
        'mcp_servers', 'mcp_server_tools', 'mcp_providers',
        'crons', 'cron_executions',
        'webhooks', 'webhook_deliveries', 'webhook_outbound_subscriptions',
        'platform_policies', 'platform_policy_versions',
        'prompt_captured', 'usage_counters', 'usage_alerts',
        'sdd_proposals', 'sdd_designs', 'sdd_requirements',
        'organizations', 'org_members',
    ];
BEGIN
    FOREACH t IN ARRAY domain_tables
    LOOP
        IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = t) THEN
            EXECUTE format('GRANT SELECT ON %I TO domain_chat_reader', t);
        END IF;
    END LOOP;
END
$$;

-- ============================================================
-- 3) Permisos: INSERT/UPDATE/DELETE en tablas de chat
--    (el bot NECESITA escribir ahi para funcionar)
-- ============================================================
GRANT SELECT, INSERT, UPDATE, DELETE ON chat_conversations TO domain_chat_reader;
GRANT SELECT, INSERT, UPDATE, DELETE ON chat_messages TO domain_chat_reader;
GRANT USAGE, SELECT ON SEQUENCE chat_messages_id_seq TO domain_chat_reader;

-- ============================================================
-- 4) Grants al app_admin (sigue teniendo control total para mantenedores)
-- ============================================================
GRANT ALL ON chat_conversations, chat_messages TO app_admin;
GRANT USAGE, SELECT ON SEQUENCE chat_messages_id_seq TO app_admin;
