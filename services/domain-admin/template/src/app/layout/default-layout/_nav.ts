import { INavData } from '@coreui/angular';

import { MAINTAINERS, SPECIAL_MAINTAINERS } from '../../views/admin-maintainers/maintainer-registry';

// HU-41.1: navItems administrativos (reemplaza los items de stock del
// template CoreUI). Los items de Platform (Cross-org) se renderizan
// condicionalmente en el template del sidebar según `activeRole()?.slug`.

// HU-41.4: submenú Mantenedores — generado desde el registry.
// Mantenibilidad: agregar un maintainer en maintainer-registry.ts lo
// agrega automáticamente al sidebar.
const maintainersChildren: INavData[] = [
  ...MAINTAINERS.map(m => ({
    name: m.title,
    url: `/admin/maintainers/${m.path}`,
    iconComponent: { name: m.icon },
  })),
  { name: '──────────', url: '#', attributes: { disabled: true, class: 'sidebar-divider' } },
  ...SPECIAL_MAINTAINERS.map(m => ({
    name: m.title,
    url: `/admin/maintainers/${m.path}`,
    iconComponent: { name: m.icon },
  })),
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
