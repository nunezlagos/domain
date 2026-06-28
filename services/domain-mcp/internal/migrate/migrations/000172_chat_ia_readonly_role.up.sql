-- migration: 000172_chat_ia_readonly_role
-- author: NunezLagos
-- issue: legacy
-- estimated_duration: unknown
-- description: crea rol `domain_chat_reader` con permisos minimos para que
--   domain-admin (chat) NO pueda escribir sobre tablas del dominio.
--   Previene LLM08 (Excessive Agency) y limita el blast radius si el
--   bot se compromete.
-- breaking: no (no cambia permisos de app_user ni app_admin; solo crea rol nuevo).
-- Aplica: domain-mcp. domain-admin debe migrar a usar este rol.

-- ============================================================
-- 1) Crear rol readonly para el chat
-- ============================================================
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'domain_chat_reader') THEN
        CREATE ROLE domain_chat_reader NOLOGIN;
    END IF;
END $$;

GRANT USAGE ON SCHEMA public TO domain_chat_reader;

-- ============================================================
-- 2) Permisos: SELECT en todas las tablas del dominio (read-only)
--    GRANTs en SQL plano (no DO block) para evitar syntax issues con
--    quoting en plpgsql.
-- ============================================================
GRANT SELECT ON agents TO domain_chat_reader;
GRANT SELECT ON agent_templates TO domain_chat_reader;
GRANT SELECT ON agent_versions TO domain_chat_reader;
GRANT SELECT ON agent_runs TO domain_chat_reader;
GRANT SELECT ON skills TO domain_chat_reader;
GRANT SELECT ON skill_versions TO domain_chat_reader;
GRANT SELECT ON skill_executions TO domain_chat_reader;
GRANT SELECT ON flows TO domain_chat_reader;
GRANT SELECT ON flow_versions TO domain_chat_reader;
GRANT SELECT ON flow_runs TO domain_chat_reader;
GRANT SELECT ON flow_run_steps TO domain_chat_reader;
GRANT SELECT ON prompts TO domain_chat_reader;
GRANT SELECT ON projects TO domain_chat_reader;
GRANT SELECT ON project_templates TO domain_chat_reader;
GRANT SELECT ON project_repositories TO domain_chat_reader;
GRANT SELECT ON project_skills TO domain_chat_reader;
GRANT SELECT ON project_tickets TO domain_chat_reader;
GRANT SELECT ON project_ticket_comments TO domain_chat_reader;
GRANT SELECT ON project_policies TO domain_chat_reader;
GRANT SELECT ON users TO domain_chat_reader;
GRANT SELECT ON roles TO domain_chat_reader;
GRANT SELECT ON user_roles TO domain_chat_reader;
GRANT SELECT ON project_clients TO domain_chat_reader;
GRANT SELECT ON issues TO domain_chat_reader;
GRANT SELECT ON knowledge_docs TO domain_chat_reader;
GRANT SELECT ON knowledge_chunks TO domain_chat_reader;
GRANT SELECT ON knowledge_observations TO domain_chat_reader;
GRANT SELECT ON mcp_servers TO domain_chat_reader;
GRANT SELECT ON mcp_server_tools TO domain_chat_reader;
GRANT SELECT ON mcp_providers TO domain_chat_reader;
GRANT SELECT ON crons TO domain_chat_reader;
GRANT SELECT ON cron_executions TO domain_chat_reader;
GRANT SELECT ON webhooks TO domain_chat_reader;
GRANT SELECT ON platform_policies TO domain_chat_reader;
GRANT SELECT ON platform_policy_versions TO domain_chat_reader;
GRANT SELECT ON prompt_captured TO domain_chat_reader;
GRANT SELECT ON usage_counters TO domain_chat_reader;
GRANT SELECT ON usage_alerts TO domain_chat_reader;

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
GRANT ALL ON chat_conversations TO app_admin;
GRANT ALL ON chat_messages TO app_admin;
GRANT USAGE, SELECT ON SEQUENCE chat_messages_id_seq TO app_admin;
