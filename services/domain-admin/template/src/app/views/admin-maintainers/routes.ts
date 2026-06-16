import { Routes } from '@angular/router';

import { MAINTAINERS, SPECIAL_MAINTAINERS } from './maintainer-registry';

// HU-41.4: rutas dinámicas. Para cada MaintainerDef en el registry se
// genera un lazy child route que monta GenericMaintainerComponent
// (data-driven, sin código custom). Los SPECIAL_MAINTAINERS (system,
// runtime-config) tienen componentes dedicados.
export const routes: Routes = [
  {
    path: '',
    loadComponent: () => import('./maintainers.component').then(m => m.MaintainersComponent),
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'users' },

      // Generic maintainers (data-driven)
      ...MAINTAINERS.map(m => ({
        path: m.path,
        loadComponent: () => import('./generic-maintainer.component').then(c => c.GenericMaintainerComponent),
        data: {
          title: m.title, icon: m.icon, description: m.description,
          endpoint: m.endpoint,
        },
      })),

      // Special maintainers (dedicated components)
      ...SPECIAL_MAINTAINERS.map(m => ({
        path: m.path,
        loadComponent: () => import(`./${m.path}.component`).then(c => (c as any)[pascal(m.path) + 'Component']),
        data: { title: m.title, icon: m.icon, description: m.description },
      })),
    ],
  },
];

function pascal(s: string): string {
  return s.split('-').map(p => p[0].toUpperCase() + p.slice(1)).join('');
}
