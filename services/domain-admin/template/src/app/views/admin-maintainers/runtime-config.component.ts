import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { FormsModule } from '@angular/forms';

import { AlertComponent, CardComponent, CardBodyComponent, ButtonDirective, FormControlDirective, InputGroupComponent, InputGroupTextDirective, BadgeComponent } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';

interface RuntimeConfig {
  key: string;
  value: any;
  source: 'env' | 'flag' | 'database' | 'default';
  description?: string;
  updated_at?: string;
}

// HU-41.4: Runtime Config — vista de la config activa del backend.
// Refleja lo que el dominio está usando AHORA (env vars, feature flags,
// defaults). Read-only por seguridad; cambios requieren redeploy.
@Component({
  selector: 'app-runtime-config',
  standalone: true,
  imports: [
    CommonModule, DatePipe, FormsModule, IconDirective,
    AlertComponent, CardComponent, CardBodyComponent, ButtonDirective, BadgeComponent,
    FormControlDirective, InputGroupComponent, InputGroupTextDirective,
  ],
  template: `
    <c-alert color="warning" class="mb-3">
      <svg cIcon name="cilWarning" class="me-2"></svg>
      <strong>Read-only.</strong> Los cambios de config requieren redeploy del backend
      o de los feature flags vía MCP. Esta vista es para inspección.
    </c-alert>

    <div class="mb-3 d-flex gap-2 align-items-center flex-wrap">
      <c-input-group style="max-width: 360px;">
        <span cInputGroupText>
          <svg cIcon name="cilSearch"></svg>
        </span>
        <input
          cFormControl
          type="search"
          placeholder="Buscar config (e.g. 'http', 'database', 'log')"
          [value]="filter()"
          (input)="filter.set($any($event.target).value)" />
      </c-input-group>
      <button cButton color="primary" (click)="load()">Cargar</button>
    </div>

    <div class="row g-3">
      @for (cfg of filteredConfigs(); track cfg.key) {
        <div class="col-md-6">
          <c-card>
            <c-card-body>
              <div class="d-flex justify-content-between align-items-start">
                <code class="fs-6">{{ cfg.key }}</code>
                <c-badge [color]="badgeColor(cfg.source)">{{ cfg.source }}</c-badge>
              </div>
              <pre class="bg-body-tertiary p-2 mt-2 mb-0 small rounded">{{ formatValue(cfg.value) }}</pre>
              @if (cfg.description) {
                <small class="text-body-secondary d-block mt-2">{{ cfg.description }}</small>
              }
            </c-card-body>
          </c-card>
        </div>
      } @empty {
        <div class="col-12">
          <c-alert color="info">
            Sin configs que matcheen el filtro. Probá "http", "database", "log", "s3", "rate".
          </c-alert>
        </div>
      }
    </div>
  `,
  styles: [`
    pre { white-space: pre-wrap; word-break: break-all; }
  `],
})
export class RuntimeConfigComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly configs = signal<RuntimeConfig[]>([]);
  readonly filter = signal('');
  readonly filteredConfigs = signal<RuntimeConfig[]>([]);

  ngOnInit() { this.load(); }

  load() {
    // Por ahora exponemos configs hardcodeadas del sistema. En el futuro
    // el endpoint /api/v1/admin/runtime-configs/{key} retornará la config
    // persistida y la combinamos con env vars.
    const defaults: RuntimeConfig[] = [
      { key: 'DOMAIN_HTTP_BIND',      value: '0.0.0.0',          source: 'env', description: 'Bind address del HTTP server' },
      { key: 'DOMAIN_HTTP_PORT',      value: 8000,               source: 'env' },
      { key: 'DOMAIN_METRICS_ENABLED', value: true,               source: 'env' },
      { key: 'DOMAIN_METRICS_PORT',   value: 9090,               source: 'env' },
      { key: 'DOMAIN_METRICS_BIND',   value: '0.0.0.0',          source: 'env' },
      { key: 'DOMAIN_DATABASE_URL',   value: 'postgres://app_user:****@postgres:5432/domain?sslmode=disable', source: 'env', description: 'Postgres URL (password redacted)' },
      { key: 'DOMAIN_DATABASE_AUTH_URL', value: 'postgres://app_admin:****@postgres:5432/domain?sslmode=disable', source: 'env', description: 'Auth pool URL (bypasea RLS)' },
      { key: 'DOMAIN_S3_ENDPOINT',    value: 'http://minio:9000', source: 'env' },
      { key: 'DOMAIN_S3_BUCKET',      value: 'domain-attachments', source: 'env' },
      { key: 'DOMAIN_S3_REGION',      value: 'us-east-1',        source: 'env' },
      { key: 'log_level',             value: 'info',             source: 'env' },
      { key: 'http_timeout',          value: 30,                 source: 'env' },
      { key: 'APP_ADMIN_PASSWORD',    value: '****',             source: 'env', description: 'Password inicial del admin (cambiar tras primer login)' },
    ];
    this.configs.set(defaults);
    this.applyFilter();
  }

  applyFilter() {
    const f = this.filter().trim().toLowerCase();
    if (!f) {
      this.filteredConfigs.set(this.configs());
      return;
    }
    this.filteredConfigs.set(
      this.configs().filter(c => c.key.toLowerCase().includes(f) || c.description?.toLowerCase().includes(f)),
    );
  }

  formatValue(v: any): string {
    if (typeof v === 'string') return v;
    return JSON.stringify(v, null, 2);
  }

  badgeColor(source: string): string {
    return source === 'env' ? 'info' : source === 'flag' ? 'warning' : source === 'database' ? 'success' : 'secondary';
  }
}
