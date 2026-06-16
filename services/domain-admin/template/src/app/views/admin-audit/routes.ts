import { Routes } from '@angular/router';
import { PlaceholderComponent } from '../placeholder/placeholder.component';

export const routes: Routes = [
  {
    path: '',
    component: PlaceholderComponent,
    data: {
      title: 'Audit',
      issue: '41.5',
      description: 'Tabla de audit log con filtros (actor, recurso, action, rango fechas, org si super_admin), export CSV, vista detalle.',
    },
  },
];
