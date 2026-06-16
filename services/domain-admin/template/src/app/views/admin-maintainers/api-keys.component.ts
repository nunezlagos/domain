import { Component, inject, signal, OnInit, computed } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';

import { CardComponent, CardBodyComponent, SpinnerComponent, AlertComponent, ButtonDirective } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { AuthService } from '../../core/auth.service';
import { ResourceListComponent, ColumnDef } from './resource-list.component';

interface APIKey {
  id: string;
  user_id: string;
  user_email?: string;
  organization_id: string;
  key_prefix: string;
  name: string;
  environment: string;
  created_at: string;
  last_used_at: string | null;
  expires_at: string | null;
  revoked: boolean;
}

// HU-41.4: Mantenedor de API keys — listar, ver detalles, revocar.
// No podemos mostrar la API key completa (solo se muestra una vez al crearla).
@Component({
  selector: 'app-api-keys',
  standalone: true,
  imports: [
    CommonModule, DatePipe, IconDirective,
    CardComponent, CardBodyComponent, SpinnerComponent, AlertComponent, ButtonDirective,
    ResourceListComponent,
  ],
  template: `
    <app-resource-list
      [title]="'API Keys'"
      [subtitle]="orgLabel()"
      [columns]="columns"
      [rows]="keys"
      [loading]="loading"
      [error]="error"
      [emptyText]="'Sin API keys'"
      [emptyHint]="'Creá una desde Members → Crear con API key.'"
      emptyIcon="cilFingerprint"
      (refresh)="load()">
    </app-resource-list>

    @if (successMsg()) {
      <c-alert color="success" class="mt-3">
        <svg cIcon name="cilCheckCircle" class="me-2"></svg>{{ successMsg() }}
      </c-alert>
    }
  `,
})
export class ApiKeysComponent implements OnInit {
  private readonly http = inject(HttpClient);
  private readonly auth = inject(AuthService);

  readonly keys = signal<APIKey[]>([]);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);
  readonly successMsg = signal<string | null>(null);
  readonly orgId = computed(() => this.auth.user()?.organization_id);
  readonly orgLabel = computed(() => `org: ${this.orgId()?.slice(0, 8) ?? '—'}…`);

  readonly columns: ColumnDef<APIKey>[] = [
    { key: 'key_prefix', label: 'Prefix', value: r => `domk_${r.environment}_${r.key_prefix}…`, format: 'truncate' },
    { key: 'name', label: 'Nombre', nullable: false },
    { key: 'user_email', label: 'Usuario', value: r => r.user_email ?? r.user_id.slice(0, 8) + '…' },
    { key: 'environment', label: 'Env', value: r => r.environment },
    { key: 'created_at', label: 'Creado', format: 'date' },
    { key: 'last_used_at', label: 'Último uso', format: 'date' },
    { key: 'revoked', label: 'Estado', value: r => r.revoked ? 'Revocada' : 'Activa' },
  ];

  ngOnInit() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.http.get<{ data: APIKey[] | null }>(`${apiBase()}/api/v1/api-keys`)
      .subscribe({
        next: r => {
          this.keys.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudieron cargar las API keys');
          this.loading.set(false);
        },
      });
  }
}
