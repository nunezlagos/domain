import { INavData } from '@coreui/angular';

// HU-41.1: navItems administrativos (reemplaza los items de stock del
// template CoreUI). Los items de Platform (Cross-org) se renderizan
// condicionalmente en el template del sidebar según `activeRole()?.slug`.

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
    name: 'Settings',
    url: '/admin/settings',
    iconComponent: { name: 'cilSettings' },
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
