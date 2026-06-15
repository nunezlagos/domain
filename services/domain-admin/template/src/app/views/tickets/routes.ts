import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    loadComponent: () => import('./tickets.component').then(m => m.TicketsComponent),
    data: { title: 'Tickets' },
  },
];
