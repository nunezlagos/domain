import { Routes } from '@angular/router';
import { PlaceholderComponent } from '../placeholder/placeholder.component';

export const routes: Routes = [
  {
    path: '',
    component: PlaceholderComponent,
    data: {
      title: 'Settings',
      issue: '41.4',
      description: 'Settings editables de la org: nombre, slug, timezone, default model, default channel, branding.',
    },
  },
];
