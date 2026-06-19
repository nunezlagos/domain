import { Routes } from '@angular/router';
import { PlaceholderComponent } from '../placeholder/placeholder.component';

export const routes: Routes = [
  {
    path: '',
    component: PlaceholderComponent,
    data: {
      title: 'Cost',
      issue: '41.8',
      description: 'Cost analytics: summary, breakdown por agent/project/model/user, forecast.',
    },
  },
];
