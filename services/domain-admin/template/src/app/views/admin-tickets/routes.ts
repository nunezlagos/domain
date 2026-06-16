import { Routes } from '@angular/router';
import { PlaceholderComponent } from '../placeholder/placeholder.component';

export const routes: Routes = [
  {
    path: '',
    component: PlaceholderComponent,
    data: {
      title: 'Tickets',
      issue: '41.7',
      description: 'Tickets CRUD: listar, crear, asignar, comentar, cambiar status, link a issues, bulk actions, integración Jira.',
    },
  },
];
