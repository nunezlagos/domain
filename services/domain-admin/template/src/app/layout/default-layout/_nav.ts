import { INavData } from '@coreui/angular';

// HU-41.1: navItems administrativos (reemplaza los items de stock del
// template CoreUI). Los items de Platform (Cross-org) se renderizan
// condicionalmente en el template del sidebar según `activeRole()?.slug`.

// HU-41.4: Mantenedores en el sidebar = solo 1 entrada (link).
// Las 35 vistas viven en /admin/maintainers y se navega con el search
// (no inflamos el sidebar).

export const navAdminItems: INavData[] = [
  {
    name: 'Dashboard',
    url: '/admin/dashboard',
    iconComponent: { name: 'cilSpeedometer' },
  },
  {
    name: 'Miembros',
    url: '/admin/members',
    iconComponent: { name: 'cilPeople' },
  },
  {
    name: 'Mantenedores',
    url: '/admin/maintainers',
    iconComponent: { name: 'cilSettings' },
  },
  {
    name: 'Base de datos',
    url: '/admin/database',
    iconComponent: { name: 'cilStorage' },
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
