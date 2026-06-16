import { Routes } from '@angular/router';
import { PlaceholderComponent } from '../placeholder/placeholder.component';

export const routes: Routes = [
  {
    path: '',
    component: PlaceholderComponent,
    data: {
      title: 'Cross-org',
      issue: '41.9',
      description: 'Solo super_admin: tabla de TODAS las orgs con métricas, system health, switcher de org, impersonation.',
    },
  },
];
