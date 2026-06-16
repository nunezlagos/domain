import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';

import { AlertComponent, BadgeComponent, ButtonDirective, SpinnerComponent } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';

interface SystemHealth {
  api: string;
  database: string;
  llm_provider: string;
  s3: string;
  uptime_seconds: number;
  version: string;
  build_commit: string;
  started_at: string;
  goroutines: number;
  memory_mb: number;
}

// HU-41.4: System — health checks, métricas runtime, config activa.
@Component({
  selector: 'app-system',
  standalone: true,
  imports: [
    CommonModule, DatePipe, IconDirective,
    AlertComponent, BadgeComponent, ButtonDirective, SpinnerComponent,
  ],
  template: `
    <c-alert color="info" class="mb-3">
      <svg cIcon name="cilInfo" class="me-2"></svg>
      Vista de <strong>health y runtime</strong>: estado de los servicios, uptime, métricas básicas.
      Para métricas detalladas: <code>http://13.140.183.236:9090/metrics</code> (Prometheus).
    </c-alert>

    @if (loading()) {
      <div class="text-center py-5">
        <c-spinner color="primary" aria-label="Cargando"></c-spinner>
        <p class="text-body-secondary mt-3">Cargando health del sistema…</p>
      </div>
    } @else if (error()) {
      <c-alert color="danger">
        <svg cIcon name="cilWarning" class="me-2"></svg>{{ error() }}
      </c-alert>
    } @else if (health()) {
      <div class="row g-3">
        @for (svc of services(); track svc.name) {
          <div class="col-md-3">
            <div class="card text-center h-100">
              <div class="card-body">
                <h6 class="text-body-secondary text-uppercase small">{{ svc.name }}</h6>
                <div class="display-6">
                  <c-badge [color]="svc.status === 'ok' ? 'success' : svc.status === 'unknown' ? 'secondary' : 'danger'">
                    {{ svc.status }}
                  </c-badge>
                </div>
                <small class="text-body-secondary d-block mt-2">{{ svc.detail }}</small>
              </div>
            </div>
          </div>
        }
      </div>

      <div class="row g-3 mt-3">
        <div class="col-md-6">
          <div class="card">
            <div class="card-body">
              <h5 class="card-title">Build info</h5>
              <table class="table table-sm mb-0">
                <tbody>
                  <tr><th>Version</th><td><code>{{ health()!.version }}</code></td></tr>
                  <tr><th>Commit</th><td><code>{{ health()!.build_commit }}</code></td></tr>
                  <tr><th>Started at</th><td>{{ health()!.started_at | date:'short' }}</td></tr>
                  <tr><th>Uptime</th><td>{{ formatUptime(health()!.uptime_seconds) }}</td></tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
        <div class="col-md-6">
          <div class="card">
            <div class="card-body">
              <h5 class="card-title">Runtime</h5>
              <table class="table table-sm mb-0">
                <tbody>
                  <tr><th>Goroutines</th><td>{{ health()!.goroutines }}</td></tr>
                  <tr><th>Memory</th><td>{{ health()!.memory_mb.toFixed(1) }} MB</td></tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    }
  `,
  styles: [`
    .display-6 { font-size: 1.5rem; }
  `],
})
export class SystemHealthComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly health = signal<SystemHealth | null>(null);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  readonly services = signal<{ name: string; status: string; detail: string }[]>([]);

  ngOnInit() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.http.get<SystemHealth>(`${apiBase()}/healthz`)
      .subscribe({
        next: r => {
          this.health.set(r);
          this.services.set([
            { name: 'API',          status: r.api,          detail: 'HTTP server' },
            { name: 'Database',     status: r.database,     detail: 'Postgres' },
            { name: 'LLM Provider', status: r.llm_provider, detail: 'Anthropic/OpenAI' },
            { name: 'S3 Storage',   status: r.s3,           detail: 'MinIO' },
          ]);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudo cargar el health');
          this.loading.set(false);
        },
      });
  }

  formatUptime(seconds: number): string {
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    return `${d}d ${h}h ${m}m`;
  }
}
