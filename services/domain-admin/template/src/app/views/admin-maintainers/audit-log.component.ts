import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { FormsModule } from '@angular/forms';

import { AlertComponent, ButtonDirective, FormControlDirective, InputGroupComponent, InputGroupTextDirective, BadgeComponent } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { ResourceListComponent, ColumnDef } from './resource-list.component';

interface AuditLog {
  id: string;
  actor_id: string | null;
  actor_email: string | null;
  action: string;
  entity_type: string;
  entity_id: string;
  origin_org_id: string;
  metadata: Record<string, any>;
  occurred_at: string;
}

// HU-41.4: Audit Log — todas las acciones registradas. Filtros por actor/tipo/org.
@Component({
  selector: 'app-audit-log',
  standalone: true,
  imports: [
    CommonModule, DatePipe, FormsModule, IconDirective,
    AlertComponent, ButtonDirective, BadgeComponent,
    FormControlDirective, InputGroupComponent, InputGroupTextDirective,
    ResourceListComponent,
  ],
  template: `
    <c-alert color="info" class="mb-3">
      <svg cIcon name="cilInfo" class="me-2"></svg>
      El <strong>audit log</strong> registra toda mutación con actor, target y timestamp.
      Defense-in-depth: auditable + firma de integridad en producción.
    </c-alert>

    <app-resource-list
      [title]="'Audit Log'"
      [subtitle]="'Últimos eventos'"
      [columns]="columns"
      [rows]="logs"
      [loading]="loading"
      [error]="error"
      [emptyText]="'Sin eventos'"
      [emptyHint]="'Ninguna acción auditada todavía.'"
      emptyIcon="cilHistory"
      (refresh)="load()">
    </app-resource-list>
  `,
})
export class AuditLogComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly logs = signal<AuditLog[]>([]);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  readonly columns: ColumnDef<AuditLog>[] = [
    { key: 'occurred_at', label: 'Cuándo', format: 'date' },
    { key: 'actor_email', label: 'Actor', value: r => r.actor_email ?? r.actor_id?.slice(0, 8) ?? 'system' },
    { key: 'action', label: 'Acción' },
    { key: 'entity_type', label: 'Entidad' },
    { key: 'entity_id', label: 'ID', value: r => r.entity_id?.slice(0, 8) + '…', format: 'truncate' },
    { key: 'origin_org_id', label: 'Org', value: r => r.origin_org_id?.slice(0, 8) + '…' },
  ];

  ngOnInit() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.http.get<{ data: AuditLog[] | null }>(`${apiBase()}/api/v1/audit-logs?limit=200`)
      .subscribe({
        next: r => {
          this.logs.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          // Backend tiene un bug pre-existente con NULL en actor_email:
          // "audit scan: can't scan into dest[9]: cannot scan NULL into *string"
          // Mostramos el error real para que el dev lo arregle.
          this.error.set(err?.error?.error?.message || 'No se pudieron cargar los eventos');
          this.loading.set(false);
        },
      });
  }
}
