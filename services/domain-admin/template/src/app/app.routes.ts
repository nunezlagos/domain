import { Routes } from '@angular/router';

import { authGuard } from './core/auth.guard';

// HU-41.1: reorganización del routing.
//   - `path: 'admin'` envuelve DefaultLayoutComponent con authGuard.
//   - Lazy load por feature (8 features; las HU 41.2-41.9 las implementan).
//   - `/login` y `/register` quedan públicos (sin guard).
//   - `/404` y `/500` como fallback.
//   - Redirect: '' → 'admin/dashboard', '**' → '404'.

export const routes: Routes = [
  {
    path: '',
    redirectTo: 'admin/dashboard',
    pathMatch: 'full',
  },
  {
    path: 'admin',
    canActivate: [authGuard],
    loadComponent: () => import('./layout').then(m => m.DefaultLayoutComponent),
    data: { title: 'Admin' },
    children: [
      {
        path: 'dashboard',
        loadChildren: () => import('./views/admin-dashboard/routes').then((m) => m.routes),
      },
      {
        path: 'members',
        loadChildren: () => import('./views/admin-members/routes').then((m) => m.routes),
      },
      {
        path: 'settings',
        loadChildren: () => import('./views/admin-settings/routes').then((m) => m.routes),
      },
      {
        path: 'usage',
        loadChildren: () => import('./views/admin-usage/routes').then((m) => m.routes),
      },
      {
        path: 'audit',
        loadChildren: () => import('./views/admin-audit/routes').then((m) => m.routes),
      },
      {
        path: 'tickets',
        loadChildren: () => import('./views/admin-tickets/routes').then((m) => m.routes),
      },
      {
        path: 'cost',
        loadChildren: () => import('./views/admin-cost/routes').then((m) => m.routes),
      },
      {
        path: 'cross-org',
        loadChildren: () => import('./views/admin-cross-org/routes').then((m) => m.routes),
      },
      // HU-41.4: Mantenedores — subtabs con todos los resource-views del MCP.
      {
        path: 'maintainers',
        loadChildren: () => import('./views/admin-maintainers/routes').then((m) => m.routes),
      },
      // HU-41.4: DB Explorer — schema completo en una página.
      {
        path: 'database',
        loadChildren: () => import('./views/admin-database/routes').then((m) => m.routes),
      },
    ],
  },
  // HU-41.7: tickets mantiene su ruta legacy /tickets para deep-links
  // existentes (el componente real está en views/tickets/ — la feature
  // admin-tickets solo agrega el drill-down).
  {
    path: 'tickets',
    canActivate: [authGuard],
    loadComponent: () => import('./layout').then(m => m.DefaultLayoutComponent),
    loadChildren: () => import('./views/tickets/routes').then((m) => m.routes),
  },
  {
    path: '404',
    loadComponent: () => import('./views/pages/page404/page404.component').then(m => m.Page404Component),
    data: { title: 'Page 404' },
  },
  {
    path: '500',
    loadComponent: () => import('./views/pages/page500/page500.component').then(m => m.Page500Component),
    data: { title: 'Page 500' },
  },
  {
    path: 'login',
    loadComponent: () => import('./views/pages/login/login.component').then(m => m.LoginComponent),
    data: { title: 'Login' },
  },
  {
    path: 'register',
    loadComponent: () => import('./views/pages/register/register.component').then(m => m.RegisterComponent),
    data: { title: 'Register Page' },
  },
  { path: '**', redirectTo: '404' },
];
