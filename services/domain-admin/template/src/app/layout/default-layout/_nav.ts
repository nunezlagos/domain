import { INavData } from '@coreui/angular';

// HU-41.1: navItems administrativos (reemplaza los items de stock del
// template CoreUI). Los items de Platform (Cross-org) se renderizan
// condicionalmente en el template del sidebar según `activeRole()?.slug`.

// HU-41.4: submenú Mantenedores con todas las pantallas de mantenimiento
// del MCP. CoreUI soporta `children` para submenús colapsables.
const maintainersChildren: INavData[] = [
  { name: 'API Keys', url: '/admin/maintainers/api-keys', iconComponent: { name: 'cilFingerprint' } },
  { name: 'Skills', url: '/admin/maintainers/skills', iconComponent: { name: 'cilBolt' } },
  { name: 'Agents', url: '/admin/maintainers/agents', iconComponent: { name: 'cilTerminal' } },
  { name: 'Flows', url: '/admin/maintainers/flows', iconComponent: { name: 'cilShareAll' } },
  { name: 'Crons', url: '/admin/maintainers/crons', iconComponent: { name: 'cilClock' } },
  { name: 'Observations', url: '/admin/maintainers/observations', iconComponent: { name: 'cilList' } },
  { name: 'Captured Prompts', url: '/admin/maintainers/prompts', iconComponent: { name: 'cilCopy' } },
  { name: 'Audit Log', url: '/admin/maintainers/audit', iconComponent: { name: 'cilHistory' } },
  { name: 'Proposals', url: '/admin/maintainers/proposals', iconComponent: { name: 'cilCheckCircle' } },
  { name: 'Projects', url: '/admin/maintainers/projects', iconComponent: { name: 'cilApps' } },
  { name: 'Knowledge', url: '/admin/maintainers/knowledge', iconComponent: { name: 'cilBook' } },
  { name: 'System', url: '/admin/maintainers/system', iconComponent: { name: 'cilSpeedometer' } },
];

export const navAdminItems: INavData[] = [
  {
    name: 'Dashboard',
    url: '/admin/dashboard',
    iconComponent: { name: 'cilSpeedometer' },
  },
  {
    name: 'Members',
    url: '/admin/members',
    iconComponent: { name: 'cilPeople' },
  },
  {
    name: 'Mantenedores',
    url: '/admin/maintainers',
    iconComponent: { name: 'cilSettings' },
    children: maintainersChildren,
  },
  {
    name: 'Settings',
    url: '/admin/settings',
    iconComponent: { name: 'cilApplicationsSettings' },
  },
  {
    name: 'Usage',
    url: '/admin/usage',
    iconComponent: { name: 'cilChart' },
  },
  {
    name: 'Audit',
    url: '/admin/audit',
    iconComponent: { name: 'cilHistory' },
  },
  {
    name: 'Tickets',
    url: '/admin/tickets',
    iconComponent: { name: 'cilTag' },
  },
  {
    name: 'Cost',
    url: '/admin/cost',
    iconComponent: { name: 'cilDollar' },
  },
];

export const navPlatformItems: INavData[] = [
  {
    title: true,
    name: 'Plataforma',
  },
  {
    name: 'Cross-org',
    url: '/admin/cross-org',
    iconComponent: { name: 'cilGlobeAlt' },
  },
];

// Mantener `navItems` exportado por compatibilidad con el DefaultLayoutComponent.
// El sidebar renderiza `navAdminItems` y, si super_admin, también `navPlatformItems`.
export const navItems: INavData[] = [...navAdminItems];
