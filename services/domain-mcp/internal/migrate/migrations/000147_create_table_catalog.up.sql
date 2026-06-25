



























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

  ('users',                          'users',        'Usuarios y RBAC',                  101),
  ('roles',                          'users',        'Usuarios y RBAC',                  102),
  ('user_roles',                     'users',        'Usuarios y RBAC',                  103),

  ('auth_sessions',                  'auth',         'Autenticación y credenciales',     201),
  ('auth_events',                    'auth',         'Autenticación y credenciales',     202),
  ('otp_codes',                      'auth',         'Autenticación y credenciales',     203),
  ('api_keys',                       'auth',         'Autenticación y credenciales',     204),
  ('secrets',                        'auth',         'Autenticación y credenciales',     205),
  ('invitations',                    'auth',         'Autenticación y credenciales',     206),
  ('org_enrollment_tokens',          'auth',         'Autenticación y credenciales',     207),

  ('agents',                         'agent',        'Agentes',                          301),
  ('agent_versions',                 'agent',        'Agentes',                          302),
  ('agent_templates',                'agent',        'Agentes',                          303),
  ('agent_runs',                     'agent',        'Agentes',                          304),
  ('agent_run_logs',                 'agent',        'Agentes',                          305),

  ('flows',                          'flow',         'Flujos y orquestación',            401),
  ('flow_versions',                  'flow',         'Flujos y orquestación',            402),
  ('flow_config',                    'flow',         'Flujos y orquestación',            403),
  ('flow_runs',                      'flow',         'Flujos y orquestación',            404),
  ('flow_run_steps',                 'flow',         'Flujos y orquestación',            405),
  ('flow_run_step_snapshots',        'flow',         'Flujos y orquestación',            406),
  ('flow_signals',                   'flow',         'Flujos y orquestación',            407),
  ('flow_run_signals_pending',       'flow',         'Flujos y orquestación',            408),

  ('skills',                         'skill',        'Skills',                           501),
  ('skill_versions',                 'skill',        'Skills',                           502),
  ('skill_executions',               'skill',        'Skills',                           503),

  ('mcp_providers',                  'mcp',          'Servidores MCP',                   601),
  ('mcp_servers',                    'mcp',          'Servidores MCP',                   602),
  ('mcp_server_tools',               'mcp',          'Servidores MCP',                   603),

  ('prompts',                        'prompt',       'Prompts',                          701),
  ('captured_prompts',               'prompt',       'Prompts',                          702),

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

  ('requirements',                   'sdd',          'SDD (especificación dirigida)',    901),
  ('proposals',                      'sdd',          'SDD (especificación dirigida)',    902),
  ('designs',                        'sdd',          'SDD (especificación dirigida)',    903),

  ('verifications',                  'tdd',          'TDD y verificación',              1001),
  ('verification_results',           'tdd',          'TDD y verificación',              1002),
  ('sabotage_records',               'tdd',          'TDD / Sabotaje',                  1003),

  ('issues',                         'issue',        'Issues / Historias de usuario',   1101),
  ('issue_drafts',                   'issue',        'Issues / Historias de usuario',   1102),
  ('issue_draft_steps_log',          'issue',        'Issues / Historias de usuario',   1103),
  ('issue_gherkin_scenarios',        'issue',        'Issues / Historias de usuario',   1104),
  ('tasks',                          'issue',        'Issues / Historias de usuario',   1105),
  ('code_references',                'issue',        'Issues / Historias de usuario',   1106),
  ('intake_payloads',                'issue',        'Issues / Historias de usuario',   1107),

  ('knowledge_docs',                 'knowledge',    'Base de conocimiento',            1201),
  ('knowledge_chunks',               'knowledge',    'Base de conocimiento',            1202),
  ('observations',                   'knowledge',    'Base de conocimiento',            1203),

  ('webhooks',                       'webhook',      'Webhooks (entrada y salida)',     1301),
  ('webhook_deliveries',             'webhook',      'Webhooks (entrada y salida)',     1302),
  ('outbound_webhook_subscriptions', 'webhook',      'Webhooks (entrada y salida)',     1303),
  ('outbound_webhook_deliveries',    'webhook',      'Webhooks (entrada y salida)',     1304),

  ('external_providers',             'external',     'Integraciones externas',          1401),
  ('external_sync_state',            'external',     'Integraciones externas',          1402),
  ('external_sync_events',           'external',     'Integraciones externas',          1403),

  ('crons',                          'cron',         'Tareas programadas',              1501),
  ('cron_executions',                'cron',         'Tareas programadas',              1502),

  ('usage_counters',                 'usage',        'Uso y cuotas',                    1601),
  ('usage_alerts',                   'usage',        'Uso y cuotas',                    1602),
  ('usage_alert_fires',              'usage',        'Uso y cuotas',                    1603),

  ('notification_deliveries',        'notification', 'Notificaciones',                  1701),

  ('selfhosted_runners',             'runner',       'Runners self-hosted',             1801),
  ('selfhosted_tasks',               'runner',       'Runners self-hosted',             1802),

  ('platform_policies',              'platform',     'Políticas de plataforma',         1901),
  ('platform_policy_versions',       'platform',     'Políticas de plataforma',         1902),

  ('file_attachments',               'file',         'Archivos adjuntos',               2001),

  ('audit_log',                      'audit',        'Auditoría y actividad',           2101),
  ('activity_log',                   'audit',        'Auditoría y actividad',           2102),

  ('seed_versions',                  'seed',         'Seeders',                         2201),

  ('schema_migrations',              'internal',     'Interno (oculto)',                9901)
ON CONFLICT (table_name) DO UPDATE
  SET grupo      = EXCLUDED.grupo,
      label      = EXCLUDED.label,
      sort_order = EXCLUDED.sort_order;

COMMIT;
