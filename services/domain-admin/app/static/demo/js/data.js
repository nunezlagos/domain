const SEGMENTS = [
  { id: 'agents',   label: 'Agents',    icon: 'robot',      color: DJ.segments[0] },
  { id: 'skills',   label: 'Skills',    icon: 'bolt',       color: DJ.segments[1] },
  { id: 'flows',    label: 'Flows',     icon: 'repeat',     color: DJ.segments[2] },
  { id: 'prompts',  label: 'Prompts',   icon: 'file-lines', color: DJ.segments[3] },
  { id: 'projects', label: 'Proyectos', icon: 'folder',     color: DJ.segments[4] },
  { id: 'users',    label: 'Usuarios',  icon: 'users',      color: DJ.segments[5] },
  { id: 'policies', label: 'Politicas', icon: 'shield',     color: DJ.segments[6] },
  { id: 'crons',    label: 'Crons',     icon: 'clock',      color: DJ.segments[7] },
];

const _MOCK_FALLBACK = {
  agents: [
    { name: 'Bot de Soporte',      slug: 'soporte-bot',   provider: 'minimax',   model: 'MiniMax-M3',          status: 'active',   calls: 847 },
    { name: 'Code Reviewer',       slug: 'code-reviewer', provider: 'anthropic', model: 'claude-sonnet-4-5',   status: 'active',   calls: 312 },
    { name: 'SDD Generator',       slug: 'sdd-generator', provider: 'openai',    model: 'gpt-4o',              status: 'inactive', calls: 0 },
  ],
  skills: [
    { name: 'Send Email',          slug: 'send-email',    type: 'mcp',  desc: 'Envia emails transaccionales',  calls: 1247, success: 98, status: 'active' },
    { name: 'Query Database',      slug: 'query-db',      type: 'code', desc: 'Queries SQL de solo lectura',   calls: 892,  success: 99, status: 'active' },
    { name: 'Web Search',          slug: 'web-search',    type: 'mcp',  desc: 'Busqueda web via Brave API',    calls: 654,  success: 68, status: 'active' },
  ],
  flows: [
    { name: 'SDD Pipeline v1',     slug: 'sdd-pipeline-v1', phases: 10, status: 'active',   runs: 12 },
    { name: 'Issue Intake',        slug: 'issue-intake',    phases: 5,  status: 'active',   runs: 47 },
  ],
  prompts: [
    { name: 'Code Review',         slug: 'code-review',   model: 'claude-sonnet-4-5', status: 'active',  uses: 312 },
    { name: 'SDD Phase: Spec',     slug: 'sdd-spec',      model: 'claude-sonnet-4-5', status: 'active',  uses: 89  },
  ],
  projects: [
    { name: 'test-kanban',         slug: 'test-kanban',   status: 'active', skills: 8, agents: 1, flows: 1 },
  ],
  users: [
    { name: 'admin@admin.com',     email: 'admin@admin.com',     role: 'admin',    status: 'active' },
    { name: 'operator@saargo.com', email: 'operator@saargo.com', role: 'operator', status: 'active' },
  ],
  policies: [
    { name: 'No PII in logs',      slug: 'no-pii-logs',     scope: 'platform', kind: 'security_rule', status: 'active' },
    { name: 'No PII in commits',   slug: 'no-pii-commits',  scope: 'platform', kind: 'security_rule', status: 'active' },
  ],
  crons: [
    { name: 'Backup diario',       slug: 'backup-diario',   schedule: '0 2 * * *',   status: 'active', last_run: 'hace 2h' },
    { name: 'Health check',        slug: 'health-check',    schedule: '*/5 * * * *', status: 'active', last_run: 'hace 1m' },
  ],
};

/* En el portal Django, window.PORTAL_DATA viene del backend (datos reales).
   En el demo estático (portal.html directo) usa el fallback con mock data.  */
const MOCK_DATA = window.PORTAL_DATA || _MOCK_FALLBACK;

const SEGMENT_SUBTITLES = {
  agents: 'Agentes LLM del sistema',
  skills: 'Habilidades reutilizables',
  flows: 'Pipelines de ejecucion',
  prompts: 'Templates de prompts',
  projects: 'Proyectos multi-tenant',
  users: 'Usuarios y roles',
  policies: 'Platform + project + skill',
  crons: 'Trabajos programados',
};

const FOCUS_CLASSES = ['hero','pos-left-1','pos-left-2','pos-left-3','pos-right-1','pos-right-2','pos-right-3','pos-bottom-1','pos-bottom-2'];
