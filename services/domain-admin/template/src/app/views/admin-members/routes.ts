import { Routes } from '@angular/router';
import { PlaceholderComponent } from '../placeholder/placeholder.component';

export const routes: Routes = [
  {
    path: '',
    component: PlaceholderComponent,
    data: {
      title: 'Members',
      issue: '41.3',
      description: 'CRUD de miembros: listar, invitar, asignar roles, revocar invitaciones, transferir ownership, ver API keys.',
    },
  },
];
