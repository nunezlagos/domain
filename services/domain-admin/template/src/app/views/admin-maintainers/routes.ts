import { Routes } from '@angular/router';

// HU-41.4: Mantenedores — vista padre con subtabs via Tab Pills (Bootstrap).
// Cada child es un resource-viewer con su propio componente.
export const routes: Routes = [
  {
    path: '',
    loadComponent: () => import('./maintainers.component').then(m => m.MaintainersComponent),
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'api-keys' },
      {
        path: 'api-keys',
        loadComponent: () => import('./api-keys.component').then(m => m.ApiKeysComponent),
        data: { title: 'API Keys', icon: 'cilFingerprint', description: 'API keys emitidas, revocación y rotación.' },
      },
      {
        path: 'skills',
        loadComponent: () => import('./skills.component').then(m => m.SkillsComponent),
        data: { title: 'Skills', icon: 'cilBolt', description: 'Skills globales y del proyecto, ejecución y registro.' },
      },
      {
        path: 'agents',
        loadComponent: () => import('./agents.component').then(m => m.AgentsComponent),
        data: { title: 'Agents', icon: 'cilTerminal', description: 'Agents registrados, versiones y ejecuciones.' },
      },
      {
        path: 'flows',
        loadComponent: () => import('./flows.component').then(m => m.FlowsComponent),
        data: { title: 'Flows', icon: 'cilShareAll', description: 'Flows DAG, runs y export/import.' },
      },
      {
        path: 'crons',
        loadComponent: () => import('./crons.component').then(m => m.CronsComponent),
        data: { title: 'Crons', icon: 'cilClock', description: 'Schedules recurrentes, enable/disable e historial.' },
      },
      {
        path: 'observations',
        loadComponent: () => import('./observations.component').then(m => m.ObservationsComponent),
        data: { title: 'Observations', icon: 'cilList', description: 'Memoria persistente (mem_save/mem_search).' },
      },
      {
        path: 'prompts',
        loadComponent: () => import('./prompts.component').then(m => m.PromptsComponent),
        data: { title: 'Captured Prompts', icon: 'cilCopy', description: 'Prompts crudos de los usuarios con metadata.' },
      },
      {
        path: 'audit',
        loadComponent: () => import('./audit-log.component').then(m => m.AuditLogComponent),
        data: { title: 'Audit Log', icon: 'cilHistory', description: 'Todas las acciones registradas con actor, target y timestamp.' },
      },
      {
        path: 'proposals',
        loadComponent: () => import('./proposals.component').then(m => m.ProposalsComponent),
        data: { title: 'Proposals', icon: 'cilCheckCircle', description: 'Skills y policies propuestas pendientes de review.' },
      },
      {
        path: 'projects',
        loadComponent: () => import('./projects.component').then(m => m.ProjectsComponent),
        data: { title: 'Projects', icon: 'cilApps', description: 'Proyectos registrados, repos y policies por proyecto.' },
      },
      {
        path: 'knowledge',
        loadComponent: () => import('./knowledge.component').then(m => m.KnowledgeComponent),
        data: { title: 'Knowledge', icon: 'cilBook', description: 'Documentos chunkeados para RAG, búsqueda semántica.' },
      },
      {
        path: 'system',
        loadComponent: () => import('./system.component').then(m => m.SystemComponent),
        data: { title: 'System', icon: 'cilSpeedometer', description: 'Health del sistema, métricas Prometheus, configuración runtime.' },
      },
    ],
  },
];
