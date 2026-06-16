import { Routes } from '@angular/router';
import { PlaceholderComponent } from '../placeholder/placeholder.component';

export const routes: Routes = [
  {
    path: '',
    component: PlaceholderComponent,
    data: {
      title: 'Usage',
      issue: '41.6',
      description: 'Vista de usage por user: para cada miembro, métricas del mes (prompts, tokens, runs, storage, cost estimado).',
    },
  },
];
