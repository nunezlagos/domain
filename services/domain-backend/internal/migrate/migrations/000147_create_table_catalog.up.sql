-- migration: create_table_catalog
-- author: mnunez@saargo.com
-- issue: REQ-42.1 (taxonomía y catálogo — source of truth)
-- description: crea la tabla `table_catalog` (source of truth de la
--   taxonomía de tablas) y la siembra con el mapeo prefijo→grupo→label
--   de las tablas CONSERVADAS, usando sus NOMBRES ACTUALES (pre-rename).
--
--   Convención: "toda tabla lleva el prefijo de su funcionalidad para
--   poder agruparla". El admin /database agrupa, ordena y etiqueta
--   leyendo de aquí (ORDER BY sort_order, table_name).
--
--   IMPORTANTE:
--   - Esta migration es ADITIVA PURA: NO renombra ni dropea nada.
--     Solo crea 1 tabla nueva y la puebla.
--   - El seed apunta a los nombres ACTUALES del schema (p.ej.
--     `org_enrollment_tokens`, `verifications`, `requirements`), NO a los
--     nombres propuestos. Los renames (HU 42.2) actualizarán table_name
--     con un UPDATE en la MISMA tx que cada ALTER TABLE RENAME.
--   - Las tablas marcadas para DROP (billing + legacy/infra) NO se
--     siembran porque desaparecen en la HU 42.3.
--   - sort_order: bloques de 100 por grupo (users=100, auth=200, ...,
--     internal=9900), +1 por tabla dentro del grupo.
--   - ON CONFLICT DO UPDATE hace el seed idempotente.
--
--   down: DROP TABLE table_catalog (atómico, derivable de este seed).
-- breaking: false (tabla nueva; no afecta API ni datos existentes)
-- estimated_duration: <1s

BEGIN;

CREATE TABLE IF NOT EXISTS table_catalog (
    table_name text PRIMARY KEY,
    grupo      text    NOT NULL,
    label      text    NOT NULL,
    sort_order integer NOT NULL
);

COMMENT ON TABLE  table_catalog IS 'Source of truth de la taxonomía de tablas (REQ-42). El admin /database agrupa, ordena y etiqueta leyendo de aquí.';
COMMENT ON COLUMN table_catalog.table_name IS 'Nombre actual de la tabla en el schema. Se actualiza en la MISMA tx que cada ALTER TABLE RENAME (HUs 42.2+).';
COMMENT ON COLUMN table_catalog.grupo      IS 'Grupo funcional (prefijo sin guion bajo final): auth, flow, issue, ...';
COMMENT ON COLUMN table_catalog.label      IS 'Etiqueta legible del grupo para la UI.';
COMMENT ON COLUMN table_catalog.sort_order IS 'Orden de presentacion. Bloque de 100 por grupo, +1 por tabla.';

INSERT INTO table_catalog (table_name, grupo, label, sort_order) VALUES
  -- users (100)
  ('users',                          'users',        'Usuarios y RBAC',                  101),
  ('roles',                          'users',        'Usuarios y RBAC',                  102),
  ('user_roles',                     'users',        'Usuarios y RBAC',                  103),
  -- auth (200)
  ('auth_sessions',                  'auth',         'Autenticación y credenciales',     201),
  ('auth_events',                    'auth',         'Autenticación y credenciales',     202),
  ('otp_codes',                      'auth',         'Autenticación y credenciales',     203),
  ('api_keys',                       'auth',         'Autenticación y credenciales',     204),
  ('secrets',                        'auth',         'Autenticación y credenciales',     205),
  ('invitations',                    'auth',         'Autenticación y credenciales',     206),
  ('org_enrollment_tokens',          'auth',         'Autenticación y credenciales',     207),
  -- agent (300)
  ('agents',                         'agent',        'Agentes',                          301),
  ('agent_versions',                 'agent',        'Agentes',                          302),
  ('agent_templates',                'agent',        'Agentes',                          303),
  ('agent_runs',                     'agent',        'Agentes',                          304),
  ('agent_run_logs',                 'agent',        'Agentes',                          305),
  -- flow (400)
  ('flows',                          'flow',         'Flujos y orquestación',            401),
  ('flow_versions',                  'flow',         'Flujos y orquestación',            402),
  ('flow_config',                    'flow',         'Flujos y orquestación',            403),
  ('flow_runs',                      'flow',         'Flujos y orquestación',            404),
  ('flow_run_steps',                 'flow',         'Flujos y orquestación',            405),
  ('flow_run_step_snapshots',        'flow',         'Flujos y orquestación',            406),
  ('flow_signals',                   'flow',         'Flujos y orquestación',            407),
  ('flow_run_signals_pending',       'flow',         'Flujos y orquestación',            408),
  -- skill (500)
  ('skills',                         'skill',        'Skills',                           501),
  ('skill_versions',                 'skill',        'Skills',                           502),
  ('skill_executions',               'skill',        'Skills',                           503),
  -- mcp (600)
  ('mcp_providers',                  'mcp',          'Servidores MCP',                   601),
  ('mcp_servers',                    'mcp',          'Servidores MCP',                   602),
  ('mcp_server_tools',               'mcp',          'Servidores MCP',                   603),
  -- prompt (700)
  ('prompts',                        'prompt',       'Prompts',                          701),
  ('captured_prompts',               'prompt',       'Prompts',                          702),
  -- project (800)
  ('projects',                       'project',      'Proyectos y tickets',              801),
  ('clients',                        'project',      'Proyectos y tickets',              802),
  ('project_templates',              'project',      'Proyectos y tickets',              803),
  ('project_repositories',           'project',      'Proyectos y tickets',              804),
  ('project_index_runs',             'project',      'Proyectos y tickets',              805),
  ('project_merges',                 'project',      'Proyectos y tickets',              806),
  ('project_policies',               'project',      'Proyectos y tickets',              807),
  ('project_policy_versions',        'project',      'Proyectos y tickets',              808),
  ('project_tickets',                'project',      'Proyectos y tickets',              809),
  ('project_ticket_comments',        'project',      'Proyectos y tickets',              810),
  ('project_ticket_status_history',  'project',      'Proyectos y tickets',              811),
  ('imported_workflow_files',        'project',      'Proyectos y tickets',              812),
  -- sdd (900)
  ('requirements',                   'sdd',          'SDD (especificación dirigida)',    901),
  ('proposals',                      'sdd',          'SDD (especificación dirigida)',    902),
  ('designs',                        'sdd',          'SDD (especificación dirigida)',    903),
  -- tdd (1000)
  ('verifications',                  'tdd',          'TDD y verificación',              1001),
  ('verification_results',           'tdd',          'TDD y verificación',              1002),
  ('sabotage_records',               'tdd',          'TDD / Sabotaje',                  1003),
  -- issue (1100)
  ('issues',                         'issue',        'Issues / Historias de usuario',   1101),
  ('issue_drafts',                   'issue',        'Issues / Historias de usuario',   1102),
  ('issue_draft_steps_log',          'issue',        'Issues / Historias de usuario',   1103),
  ('gherkin_scenarios',              'issue',        'Issues / Historias de usuario',   1104),
  ('tasks',                          'issue',        'Issues / Historias de usuario',   1105),
  ('code_references',                'issue',        'Issues / Historias de usuario',   1106),
  ('intake_payloads',                'issue',        'Issues / Historias de usuario',   1107),
  -- knowledge (1200)
  ('knowledge_docs',                 'knowledge',    'Base de conocimiento',            1201),
  ('knowledge_chunks',               'knowledge',    'Base de conocimiento',            1202),
  ('observations',                   'knowledge',    'Base de conocimiento',            1203),
  -- webhook (1300)
  ('webhooks',                       'webhook',      'Webhooks (entrada y salida)',     1301),
  ('webhook_deliveries',             'webhook',      'Webhooks (entrada y salida)',     1302),
  ('outbound_webhook_subscriptions', 'webhook',      'Webhooks (entrada y salida)',     1303),
  ('outbound_webhook_deliveries',    'webhook',      'Webhooks (entrada y salida)',     1304),
  -- external (1400)
  ('external_providers',             'external',     'Integraciones externas',          1401),
  ('external_sync_state',            'external',     'Integraciones externas',          1402),
  ('external_sync_events',           'external',     'Integraciones externas',          1403),
  -- cron (1500)
  ('crons',                          'cron',         'Tareas programadas',              1501),
  ('cron_executions',                'cron',         'Tareas programadas',              1502),
  -- usage (1600)
  ('usage_counters',                 'usage',        'Uso y cuotas',                    1601),
  ('usage_alerts',                   'usage',        'Uso y cuotas',                    1602),
  ('usage_alert_fires',              'usage',        'Uso y cuotas',                    1603),
  -- notification (1700)
  ('notification_deliveries',        'notification', 'Notificaciones',                  1701),
  -- runner (1800)
  ('selfhosted_runners',             'runner',       'Runners self-hosted',             1801),
  ('selfhosted_tasks',               'runner',       'Runners self-hosted',             1802),
  -- platform (1900)
  ('platform_policies',              'platform',     'Políticas de plataforma',         1901),
  ('platform_policy_versions',       'platform',     'Políticas de plataforma',         1902),
  -- file (2000)
  ('file_attachments',               'file',         'Archivos adjuntos',               2001),
  -- audit (2100)
  ('audit_log',                      'audit',        'Auditoría y actividad',           2101),
  ('activity_log',                   'audit',        'Auditoría y actividad',           2102),
  -- seed (2200)
  ('seed_versions',                  'seed',         'Seeders',                         2201),
  -- internal (9900) — oculta en el admin
  ('schema_migrations',              'internal',     'Interno (oculto)',                9901)
ON CONFLICT (table_name) DO UPDATE
  SET grupo      = EXCLUDED.grupo,
      label      = EXCLUDED.label,
      sort_order = EXCLUDED.sort_order;

COMMIT;
