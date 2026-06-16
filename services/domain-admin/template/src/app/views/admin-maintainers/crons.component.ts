import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';

import { AlertComponent, BadgeComponent } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { ResourceListComponent, ColumnDef } from './resource-list.component';

interface Cron {
  id: string;
  name: string;
  slug: string;
  description: string;
  cron_expression: string;
  timezone: string;
  target_type: 'flow' | 'agent' | 'skill';
  target_id: string;
  enabled: boolean;
  next_run_at: string | null;
  last_run_at: string | null;
  created_at: string;
}

// HU-41.4: Mantenedor de crons — listar, enable/disable, ver historial.
@Component({
  selector: 'app-crons',
  standalone: true,
  imports: [
    CommonModule, DatePipe, IconDirective, AlertComponent, BadgeComponent,
    ResourceListComponent,
  ],
  template: `
    <c-alert color="info" class="mb-3">
      <svg cIcon name="cilInfo" class="me-2"></svg>
      Los <strong>crons</strong> ejecutan flows/agents/skills en schedule.
      El scheduler interno polla cada 30s y dispara lo vencido.
    </c-alert>

    <app-resource-list
      [title]="'Crons programados'"
      [columns]="columns"
      [rows]="crons"
      [loading]="loading"
      [error]="error"
      [emptyText]="'Sin crons'"
      [emptyHint]="'domain_cron_create para programar uno.'"
      emptyIcon="cilClock"
      (refresh)="load()">
    </app-resource-list>
  `,
})
export class CronsComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly crons = signal<Cron[]>([]);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  readonly columns: ColumnDef<Cron>[] = [
    { key: 'name', label: 'Nombre', nullable: false },
    { key: 'cron_expression', label: 'Expresión' },
    { key: 'timezone', label: 'TZ' },
    { key: 'target_type', label: 'Target' },
    { key: 'enabled', label: 'Estado', value: r => r.enabled ? 'Activo' : 'Pausado' },
    { key: 'last_run_at', label: 'Última ejecución', format: 'date' },
    { key: 'next_run_at', label: 'Próxima', format: 'date' },
  ];

  ngOnInit() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.http.get<{ data: Cron[] | null }>(`${apiBase()}/api/v1/crons`)
      .subscribe({
        next: r => {
          this.crons.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudieron cargar los crons');
          this.loading.set(false);
        },
      });
  }
}
