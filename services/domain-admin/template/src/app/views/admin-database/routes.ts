import { Routes } from '@angular/router';

export const routes: Routes = [
  { path: '', loadComponent: () => import('./database-explorer.component').then(m => m.DatabaseExplorerComponent) },
];
