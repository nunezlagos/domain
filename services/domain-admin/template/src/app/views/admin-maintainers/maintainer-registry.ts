// HU-41.4: registry de TODOS los mantenedores del MCP.
// Cada entry mapea a un endpoint del backend + columnas a renderizar.
// El componente GenericMaintainerComponent lee este registry y renderiza
// cualquier entry sin necesidad de código custom.
//
// Cuando agregues un endpoint nuevo al backend, agregá la entry acá
// y queda automáticamente disponible como subtab.

export interface ColumnDef {
  /** Key en el row JSON, o path con dots para nested. */
  key: string;
  label: string;
  /** Función que retorna el valor a mostrar. Si no se setea, usa row[key]. */
  value?: (row: any) => any;
  format?: 'date' | 'truncate' | 'badge' | 'code' | 'json';
  /** Color para badge format: 'success' | 'danger' | 'warning' | 'info' | 'secondary'. */
  badgeColor?: (val: any) => string;
  /** True si la celda puede ser null (muestra '—'). Default true. */
  nullable?: boolean;
  width?: number;
}

export interface MaintainerDef {
  path: string;
  title: string;
  icon: string;
  description: string;
  endpoint: string;
  columns: ColumnDef[];
  /** Default query params (e.g. project_slug=domain-services). */
  defaultParams?: Record<string, string>;
  /** Si true, requiere un project_slug input. */
  requireProject?: boolean;
  /** Si true, requiere un search query input. */
  hasSearch?: boolean;
  /** Si true, requiere un ID input (para endpoints /resource/{id}). */
  requireId?: string;
  /** Categoría (para agrupar visualmente en el sidebar). */
  category: 'core' | 'resources' | 'observability' | 'system' | 'sdd' | 'ops';
}

const truncate = (s: any, n = 40) => s && s.length > n ? s.slice(0, n) + '…' : s;
const dateOrNull = (s: any) => s ? new Date(s).toLocaleString() : '—';
const badge = (color: string) => (val: any) => val ? color : 'secondary';
const statusColor = (val: any) => {
  if (val === true || val === 'active' || val === 'enabled' || val === 'ok') return 'success';
  if (val === false || val === 'inactive' || val === 'disabled' || val === 'error' || val === 'revoked') return 'danger';
  if (val === 'pending' || val === 'paused') return 'warning';
  return 'secondary';
};

const COMMON_PROJECT = 'domain-services';

export const MAINTAINERS: MaintainerDef[] = [
  // === CORE (autenticación, identidad, sesiones) ===
  {
    path: 'users', title: 'Users', icon: 'cilUser', category: 'core',
    description: 'Users del sistema (cross-org, requiere super_admin para ver otros).',
    endpoint: '/api/v1/users',
    columns: [
      { key: 'email', label: 'Email', nullable: false },
      { key: 'name', label: 'Nombre' },
      { key: 'role', label: 'Rol' },
      { key: 'organization_id', label: 'Organización', value: r => truncate(r.organization_id, 12) },
      { key: 'created_at', label: 'Creado', format: 'date' },
    ],
  },
  {
    path: 'api-keys', title: 'API Keys', icon: 'cilFingerprint', category: 'core',
    description: 'API keys emitidas. Solo se muestra una vez al crearla.',
    endpoint: '/api/v1/api-keys',
    columns: [
      { key: 'key_prefix', label: 'Prefijo', value: r => `domk_${r.environment}_${r.key_prefix}…`, format: 'truncate' },
      { key: 'name', label: 'Nombre', nullable: false },
      { key: 'user_email', label: 'Usuario', value: r => r.user_email ?? truncate(r.user_id, 8) },
      { key: 'environment', label: 'Entorno' },
      { key: 'created_at', label: 'Creado', format: 'date' },
      { key: 'last_used_at', label: 'Último uso', format: 'date' },
      { key: 'revoked', label: 'Estado', value: r => r.revoked ? 'Revocada' : 'Activa',
        format: 'badge', badgeColor: statusColor },
    ],
  },
  {
    path: 'sessions', title: 'Sessions', icon: 'cilClock', category: 'core',
    description: 'Sessions de agentes (captura de prompts en flows multi-step).',
    endpoint: '/api/v1/sessions',
    defaultParams: { project_slug: COMMON_PROJECT },
    requireProject: true,
    columns: [
      { key: 'id', label: 'ID', value: r => truncate(r.id, 12) },
      { key: 'title', label: 'Título' },
      { key: 'user_id', label: 'Usuario', value: r => truncate(r.user_id, 8) },
      { key: 'started_at', label: 'Inicio', format: 'date' },
      { key: 'ended_at', label: 'Fin', format: 'date' },
    ],
  },

  // === RESOURCES (skills, agents, flows, crons, prompts) ===
  {
    path: 'skills', title: 'Skills', icon: 'cilBolt', category: 'resources',
    description: 'Skills globales y del proyecto. Ejecución con domain_skill_execute.',
    endpoint: '/api/v1/skills',
    hasSearch: true,
    columns: [
      { key: 'name', label: 'Nombre', nullable: false },
      { key: 'slug', label: 'Slug' },
      { key: 'skill_type', label: 'Tipo' },
      { key: 'description', label: 'Descripción', format: 'truncate' },
      { key: 'version', label: 'Versión' },
      { key: 'project_id', label: 'Proyecto', value: r => r.project_id ? truncate(r.project_id, 8) : 'global' },
      { key: 'updated_at', label: 'Actualizado', format: 'date' },
    ],
  },
  {
    path: 'prompts', title: 'Prompts', icon: 'cilCopy', category: 'resources',
    description: 'Prompt templates versionados. Escribí un query y apretá Buscar.',
    endpoint: '/api/v1/prompts/search',
    hasSearch: true,
    columns: [
      { key: 'slug', label: 'Slug', nullable: false },
      { key: 'name', label: 'Nombre' },
      { key: 'version', label: 'Versión' },
      { key: 'description', label: 'Descripción', format: 'truncate' },
      { key: 'updated_at', label: 'Actualizado', format: 'date' },
    ],
  },
  {
    path: 'agents', title: 'Agents', icon: 'cilTerminal', category: 'resources',
    description: 'Agents registrados. Ejecución con domain_agent_run.',
    endpoint: '/api/v1/agents',
    columns: [
      { key: 'name', label: 'Nombre', nullable: false },
      { key: 'slug', label: 'Slug' },
      { key: 'provider', label: 'Provider' },
      { key: 'model', label: 'Modelo' },
      { key: 'description', label: 'Descripción', format: 'truncate' },
      { key: 'version', label: 'Versión' },
      { key: 'updated_at', label: 'Actualizado', format: 'date' },
    ],
  },
  {
    path: 'flows', title: 'Flows', icon: 'cilShareAll', category: 'resources',
    description: 'Flows DAG. Ejecución con domain_flow_run.',
    endpoint: '/api/v1/flows',
    columns: [
      { key: 'name', label: 'Nombre', nullable: false },
      { key: 'description', label: 'Descripción', format: 'truncate' },
      { key: 'spec_version', label: 'Espec' },
      { key: 'steps_count', label: 'Pasos' },
      { key: 'updated_at', label: 'Actualizado', format: 'date' },
    ],
  },
  {
    path: 'crons', title: 'Crons', icon: 'cilClock', category: 'resources',
    description: 'Schedules recurrentes. Enable/disable con domain_cron_set_enabled.',
    endpoint: '/api/v1/crons',
    columns: [
      { key: 'name', label: 'Nombre', nullable: false },
      { key: 'cron_expression', label: 'Expresión' },
      { key: 'timezone', label: 'Zona horaria' },
      { key: 'target_type', label: 'Target' },
      { key: 'enabled', label: 'Estado', value: r => r.enabled ? 'Activo' : 'Pausado',
        format: 'badge', badgeColor: statusColor },
      { key: 'last_run_at', label: 'Última ejecución', format: 'date' },
      { key: 'next_run_at', label: 'Próxima', format: 'date' },
    ],
  },
  {
    path: 'webhooks-in', title: 'Inbound Webhooks', icon: 'cilArrowRight', category: 'resources',
    description: '⚠️ Backend bug: nil pointer en webhook service. Ver ticket.',
    endpoint: '/api/v1/inbound-webhooks',
    columns: [
      { key: 'name', label: 'Nombre' },
      { key: 'url', label: 'URL', format: 'truncate' },
      { key: 'enabled', label: 'Habilitado', value: r => r.enabled ? 'Sí' : 'No',
        format: 'badge', badgeColor: statusColor },
      { key: 'created_at', label: 'Creado', format: 'date' },
    ],
  },
  {
    path: 'webhooks-out', title: 'Outbound Webhooks', icon: 'cilArrowLeft', category: 'resources',
    description: 'Webhooks salientes para notificaciones a sistemas externos.',
    endpoint: '/api/v1/outbound-webhooks',
    columns: [
      { key: 'name', label: 'Nombre' },
      { key: 'url', label: 'URL', format: 'truncate' },
      { key: 'enabled', label: 'Habilitado', value: r => r.enabled ? 'Sí' : 'No',
        format: 'badge', badgeColor: statusColor },
      { key: 'created_at', label: 'Creado', format: 'date' },
    ],
  },
  {
    path: 'mcp-servers', title: 'MCP Servers', icon: 'cilTerminal', category: 'resources',
    description: 'Servidores MCP registrados. Tools via domain_mcp_* o wireup.',
    endpoint: '/api/v1/mcp-servers',
    columns: [
      { key: 'name', label: 'Nombre' },
      { key: 'transport', label: 'Transporte' },
      { key: 'url', label: 'URL', format: 'truncate' },
      { key: 'enabled', label: 'Habilitado', value: r => r.enabled ? 'Sí' : 'No',
        format: 'badge', badgeColor: statusColor },
      { key: 'created_at', label: 'Creado', format: 'date' },
    ],
  },
  {
    path: 'clients', title: 'Clients (Mandantes)', icon: 'cilBriefcase', category: 'resources',
    description: 'Clientes/mandantes de la org (multi-tenant scoping).',
    endpoint: '/api/v1/clients',
    columns: [
      { key: 'name', label: 'Nombre', nullable: false },
      { key: 'slug', label: 'Slug' },
      { key: 'tax_id', label: 'Tax ID' },
      { key: 'contact_email', label: 'Email' },
      { key: 'created_at', label: 'Creado', format: 'date' },
    ],
  },
  {
    path: 'attachments', title: 'Attachments', icon: 'cilPaperclip', category: 'resources',
    description: 'Archivos subidos al S3/MinIO. Requiere entity_type + entity_id.',
    endpoint: '/api/v1/attachments?entity_type=organization&entity_id=a29bfc3c-e758-4557-9737-f251c90eb643',
    columns: [
      { key: 'filename', label: 'Nombre archivo', nullable: false },
      { key: 'content_type', label: 'Tipo de contenido' },
      { key: 'size_bytes', label: 'Tamaño', value: r => r.size_bytes ? `${(r.size_bytes/1024).toFixed(1)}KB` : '—' },
      { key: 'created_at', label: 'Subido', format: 'date' },
    ],
  },

  // === OBSERVABILITY (audit, memory, prompts capturados) ===
  {
    path: 'audit-log', title: 'Audit Log', icon: 'cilHistory', category: 'observability',
    description: 'Todas las acciones registradas con actor, target y timestamp.',
    endpoint: '/api/v1/audit-logs?limit=200',
    columns: [
      { key: 'occurred_at', label: 'Cuándo', format: 'date' },
      { key: 'actor_email', label: 'Actor', value: r => r.actor_email ?? truncate(r.actor_id, 8) ?? 'system' },
      { key: 'action', label: 'Acción' },
      { key: 'entity_type', label: 'Entidad' },
      { key: 'entity_id', label: 'ID', value: r => truncate(r.entity_id, 12) },
      { key: 'origin_org_id', label: 'Organización', value: r => truncate(r.origin_org_id, 8) },
    ],
  },
  {
    path: 'activity-logs', title: 'Activity Logs', icon: 'cilList', category: 'observability',
    description: 'Activity stream de usuarios (login, actions, etc).',
    endpoint: '/api/v1/activity-logs',
    columns: [
      { key: 'occurred_at', label: 'Cuándo', format: 'date' },
      { key: 'actor_email', label: 'Actor', value: r => r.actor_email ?? 'system' },
      { key: 'action', label: 'Acción' },
      { key: 'resource_type', label: 'Recurso' },
    ],
  },
  {
    path: 'observations', title: 'Observations (Memory)', icon: 'cilList', category: 'observability',
    description: 'Memoria persistente cross-session (domain_mem_save / mem_search).',
    endpoint: '/api/v1/observations',
    requireProject: true,
    defaultParams: { project_slug: COMMON_PROJECT },
    columns: [
      { key: 'observation_type', label: 'Tipo' },
      { key: 'project_slug', label: 'Proyecto' },
      { key: 'content', label: 'Contenido', format: 'truncate' },
      { key: 'tags', label: 'Tags', value: r => (r.tags || []).join(', '), nullable: true },
      { key: 'updated_at', label: 'Actualizado', format: 'date' },
    ],
  },
  {
    path: 'captured-prompts', title: 'Captured Prompts', icon: 'cilCopy', category: 'observability',
    description: 'Prompts crudos de los usuarios con metadata y token counts.',
    endpoint: '/api/v1/captured-prompts',
    columns: [
      { key: 'content', label: 'Prompt', format: 'truncate' },
      { key: 'project_slug', label: 'Proyecto' },
      { key: 'model', label: 'Modelo' },
      { key: 'char_count', label: 'Chars' },
      { key: 'estimated_tokens', label: 'Tokens (est.)' },
      { key: 'client_kind', label: 'Cliente' },
      { key: 'captured_at', label: 'Capturado', format: 'date' },
    ],
  },
  {
    path: 'dlq', title: 'Dead Letter Queue', icon: 'cilWarning', category: 'observability',
    description: 'Items fallidos que requieren intervención manual.',
    endpoint: '/api/v1/dlq',
    columns: [
      { key: 'id', label: 'ID', value: r => truncate(r.id, 12) },
      { key: 'reason', label: 'Razón', format: 'truncate' },
      { key: 'source', label: 'Source' },
      { key: 'attempts', label: 'Intentos' },
      { key: 'created_at', label: 'Cuándo', format: 'date' },
    ],
  },

  // === SYSTEM (knowledge, policies, projects, costs) ===
  {
    path: 'projects', title: 'Projects', icon: 'cilApps', category: 'system',
    description: 'Proyectos registrados. Scoping: knowledge, observations, policies.',
    endpoint: '/api/v1/projects',
    columns: [
      { key: 'name', label: 'Nombre', nullable: false },
      { key: 'slug', label: 'Slug' },
      { key: 'client_slug', label: 'Cliente', value: r => r.client_slug ?? '—' },
      { key: 'repository_url', label: 'Repo', value: r => r.repository_url?.replace(/^https?:\/\//, ''), format: 'truncate' },
      { key: 'branch_default', label: 'Branch' },
      { key: 'updated_at', label: 'Actualizado', format: 'date' },
    ],
  },
  {
    path: 'project-templates', title: 'Project Templates', icon: 'cilLayers', category: 'system',
    description: 'Templates reusables para crear projects nuevos.',
    endpoint: '/api/v1/project-templates',
    columns: [
      { key: 'name', label: 'Nombre' },
      { key: 'slug', label: 'Slug' },
      { key: 'description', label: 'Descripción', format: 'truncate' },
    ],
  },
  {
    path: 'knowledge', title: 'Knowledge', icon: 'cilBook', category: 'system',
    description: 'Documentos chunkeados para RAG. Búsqueda semántica + BM25.',
    endpoint: '/api/v1/knowledge',
    requireProject: true,
    defaultParams: { project_slug: COMMON_PROJECT },
    hasSearch: true,
    columns: [
      { key: 'title', label: 'Título', nullable: false },
      { key: 'project_slug', label: 'Proyecto', value: r => r.project_slug ?? '—' },
      { key: 'source', label: 'Source' },
      { key: 'chunk_count', label: 'Chunks' },
      { key: 'tags', label: 'Tags', value: r => (r.tags || []).join(', '), nullable: true },
      { key: 'updated_at', label: 'Actualizado', format: 'date' },
    ],
  },
  {
    path: 'platform-policies', title: 'Platform Policies', icon: 'cilLockLocked', category: 'system',
    description: 'Policies globales de la org (convención, security, architecture, etc).',
    endpoint: '/api/v1/platform/policies',
    columns: [
      { key: 'slug', label: 'Slug', nullable: false },
      { key: 'name', label: 'Nombre' },
      { key: 'kind', label: 'Tipo' },
      { key: 'is_active', label: 'Activa', value: r => r.is_active ? 'Sí' : 'No',
        format: 'badge', badgeColor: statusColor },
      { key: 'source', label: 'Source' },
      { key: 'updated_at', label: 'Actualizado', format: 'date' },
    ],
  },
  {
    path: 'cost-daily', title: 'Cost (Diario)', icon: 'cilDollar', category: 'system',
    description: 'Costos agregados por día.',
    endpoint: '/api/v1/cost/daily?days=30',
    columns: [
      { key: 'date', label: 'Fecha', format: 'date' },
      { key: 'cost_usd', label: 'USD', value: r => `$${(r.cost_usd ?? 0).toFixed(4)}` },
      { key: 'tokens_in', label: 'Tokens in' },
      { key: 'tokens_out', label: 'Tokens out' },
    ],
  },
  {
    path: 'usage-alerts', title: 'Usage Alerts', icon: 'cilBell', category: 'system',
    description: 'Alertas configuradas para uso de tokens/cost.',
    endpoint: '/api/v1/usage-alerts',
    columns: [
      { key: 'name', label: 'Nombre' },
      { key: 'metric', label: 'Métrica' },
      { key: 'threshold', label: 'Threshold' },
      { key: 'enabled', label: 'Habilitado', value: r => r.enabled ? 'Sí' : 'No',
        format: 'badge', badgeColor: statusColor },
    ],
  },

  // === SDD (specification-driven development) ===
  {
    path: 'requirements', title: 'Requirements', icon: 'cilFile', category: 'sdd',
    description: 'REQ-XX: requisitos funcionales de alto nivel.',
    endpoint: '/api/v1/requirements',
    columns: [
      { key: 'slug', label: 'Slug', nullable: false },
      { key: 'name', label: 'Nombre' },
      { key: 'description', label: 'Descripción', format: 'truncate' },
      { key: 'h_us_count', label: 'HUs' },
      { key: 'status', label: 'Status' },
      { key: 'updated_at', label: 'Actualizado', format: 'date' },
    ],
  },
  {
    path: 'user-stories', title: 'User Stories (HUs)', icon: 'cilBookmark', category: 'sdd',
    description: 'HU-XX.Y: historias de usuario con Gherkin scenarios.',
    endpoint: '/api/v1/user-stories',
    columns: [
      { key: 'slug', label: 'Slug', nullable: false },
      { key: 'requirement_slug', label: 'REQ' },
      { key: 'title', label: 'Título' },
      { key: 'status', label: 'Status', format: 'badge', badgeColor: statusColor },
      { key: 'priority', label: 'Prioridad' },
      { key: 'updated_at', label: 'Actualizado', format: 'date' },
    ],
  },
  {
    path: 'hu-drafts', title: 'HU Drafts (wizard)', icon: 'cilPencil', category: 'sdd',
    description: 'Borradores de HUs en wizard de creación.',
    endpoint: '/api/v1/hu-drafts',
    columns: [
      { key: 'id', label: 'ID', value: r => truncate(r.id, 12) },
      { key: 'title', label: 'Título' },
      { key: 'status', label: 'Status' },
      { key: 'created_at', label: 'Creado', format: 'date' },
    ],
  },
  {
    path: 'proposals', title: 'Proposals (pendientes review)', icon: 'cilCheckCircle', category: 'sdd',
    description: 'Skills y policies generadas por LLM. Requieren review humano.',
    endpoint: '/api/v1/proposals',
    columns: [
      { key: 'kind', label: 'Tipo' },
      { key: 'name', label: 'Nombre', nullable: false },
      { key: 'slug', label: 'Slug' },
      { key: 'project_slug', label: 'Proyecto', value: r => r.project_slug ?? 'global' },
      { key: 'source', label: 'Source' },
      { key: 'rationale', label: 'Rationale', format: 'truncate' },
      { key: 'created_at', label: 'Creado', format: 'date' },
    ],
  },

  // === OPS (admin/debug) ===
  {
    path: 'admin-db-stats', title: 'DB Stats', icon: 'cilStorage', category: 'ops',
    description: 'Métricas de la DB: tamaño, conexiones, cache hit rate.',
    endpoint: '/api/v1/admin/db-stats',
    columns: [
      { key: 'metric', label: 'Métrica' },
      { key: 'value', label: 'Valor' },
      { key: 'unit', label: 'Unidad' },
    ],
  },
  {
    path: 'admin-slow-queries', title: 'Slow Queries', icon: 'cilSpeedometer', category: 'ops',
    description: 'Queries más lentas detectadas por pg_stat_statements.',
    endpoint: '/api/v1/admin/db/slow-queries',
    columns: [
      { key: 'query', label: 'Query', format: 'truncate' },
      { key: 'calls', label: 'Calls' },
      { key: 'mean_time_ms', label: 'Mean (ms)' },
      { key: 'total_time_ms', label: 'Total (ms)' },
    ],
  },
  {
    path: 'tickets', title: 'Tickets', icon: 'cilTag', category: 'ops',
    description: 'Tickets internos del sistema (issues, tasks, bugs).',
    endpoint: '/api/v1/tickets',
    columns: [
      { key: 'key', label: 'Key', nullable: false },
      { key: 'title', label: 'Título' },
      { key: 'issue_type', label: 'Tipo' },
      { key: 'status', label: 'Status', format: 'badge', badgeColor: statusColor },
      { key: 'priority', label: 'Prioridad' },
      { key: 'project_slug', label: 'Proyecto' },
      { key: 'updated_at', label: 'Actualizado', format: 'date' },
    ],
  },
];

// === Mantenedores especiales (no son listas simples) ===
// El registry de arriba es para listas genéricas. Los especiales van
// en components dedicados pero se referencian acá para que aparezcan
// en el sidebar.
export const SPECIAL_MAINTAINERS = [
  { path: 'system-health', title: 'System Health', icon: 'cilHeart', description: 'Health, build, runtime, config.' },
  { path: 'runtime-config', title: 'Runtime Config', icon: 'cilSettings', description: 'Config activa del backend (read-only).' },
];
