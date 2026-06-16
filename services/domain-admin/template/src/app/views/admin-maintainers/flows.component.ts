import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';

import { AlertComponent } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { ResourceListComponent, ColumnDef } from './resource-list.component';

interface Flow {
  id: string;
  name: string;
  description: string;
  spec_version: number;
  steps_count: number;
  project_id: string | null;
  created_at: string;
  updated_at: string;
}

// HU-41.4: Mantenedor de flows — listar, ver runs, export/import.
@Component({
  selector: 'app-flows',
  standalone: true,
  imports: [
    CommonModule, DatePipe, IconDirective,
    AlertComponent, ResourceListComponent,
  ],
  template: `
    <c-alert color="info" class="mb-3">
      <svg cIcon name="cilInfo" class="me-2"></svg>
      Los <strong>flows</strong> son DAGs de steps. Se ejecutan con <code>domain_flow_run</code>.
    </c-alert>

    <app-resource-list
      [title]="'Flows registrados'"
      [columns]="columns"
      [rows]="flows"
      [loading]="loading"
      [error]="error"
      [emptyText]="'Sin flows'"
      [emptyHint]="'domain_flow_create para registrar uno nuevo.'"
      emptyIcon="cilShareAll"
      (refresh)="load()">
    </app-resource-list>
  `,
})
export class FlowsComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly flows = signal<Flow[]>([]);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  readonly columns: ColumnDef<Flow>[] = [
    { key: 'name', label: 'Nombre', nullable: false },
    { key: 'description', label: 'Descripción', format: 'truncate' },
    { key: 'spec_version', label: 'Spec' },
    { key: 'steps_count', label: 'Steps' },
    { key: 'updated_at', label: 'Actualizado', format: 'date' },
  ];

  ngOnInit() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.http.get<{ data: Flow[] | null }>(`${apiBase()}/api/v1/flows`)
      .subscribe({
        next: r => {
          this.flows.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudieron cargar los flows');
          this.loading.set(false);
        },
      });
  }
}
