import { Routes } from '@angular/router';
import { PlaceholderComponent } from '../placeholder/placeholder.component';

export const routes: Routes = [
  {
    path: '',
    component: PlaceholderComponent,
    data: {
      title: 'Dashboard',
      issue: '41.2',
      description: 'Vista home del admin: stat cards, top users del mes, actividad reciente, system health (super_admin), acciones rápidas.',
    },
  },
];
