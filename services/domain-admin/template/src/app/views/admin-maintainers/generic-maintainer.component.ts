import { Component, inject, signal, computed, OnInit, Input } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';

import {
  CardComponent, CardBodyComponent, CardHeaderComponent,
  SpinnerComponent, AlertComponent, BadgeComponent,
  ButtonDirective, FormControlDirective, InputGroupComponent, InputGroupTextDirective,
} from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { MaintainerDef, ColumnDef, MAINTAINERS } from './maintainer-registry';

// HU-41.4: Mantenedor genérico data-driven. Lee el registry por path
// y hace la query + render sin código custom. Usado por ~30 maintainers
// de listas. Los especiales (system, runtime-config) tienen su propio componente.

@Component({
  selector: 'app-generic-maintainer',
  standalone: true,
  imports: [
    CommonModule, DatePipe, FormsModule, IconDirective,
    CardComponent, CardBodyComponent, CardHeaderComponent,
    SpinnerComponent, AlertComponent, BadgeComponent,
    ButtonDirective, FormControlDirective, InputGroupComponent, InputGroupTextDirective,
  ],
  template: `
    @if (defValue; as def) {
      <c-alert color="info" class="mb-3">
        <svg cIcon [name]="def.icon" class="me-2"></svg>
        {{ def.description }}
        <small class="text-body-secondary ms-2">
          <code>GET {{ def.endpoint }}</code>
        </small>
      </c-alert>

      <!-- Filter bar (project, search) -->
      @if (def.requireProject || def.hasSearch) {
        <div class="mb-3 d-flex gap-2 align-items-center flex-wrap">
          @if (def.requireProject) {
            <c-input-group style="max-width: 240px;">
              <span cInputGroupText>
                <svg cIcon name="cilApps"></svg>
              </span>
              <input
                cFormControl
                type="text"
                placeholder="project_slug"
                [value]="projectFilter()"
                (input)="projectFilter.set($any($event.target).value)" />
            </c-input-group>
          }
          @if (def.hasSearch) {
            <c-input-group style="max-width: 360px;">
              <span cInputGroupText>
                <svg cIcon name="cilSearch"></svg>
              </span>
              <input
                cFormControl
                type="search"
                placeholder="Buscar…"
                [value]="searchQuery()"
                (input)="searchQuery.set($any($event.target).value)" />
            </c-input-group>
          }
          <button cButton color="primary" (click)="load()">
            <svg cIcon name="cilList" class="me-1"></svg> Cargar
          </button>
        </div>
      }

      <c-card>
        <c-card-header class="d-flex justify-content-between align-items-center">
          <div>
            <svg [cIcon]="def.icon" class="me-2"></svg>
            <strong>{{ def.title }}</strong>
          </div>
          <div class="d-flex gap-2 align-items-center">
            <span class="badge bg-primary">{{ rows().length }}</span>
            <button cButton color="secondary" variant="outline" size="sm" (click)="load()">
              <svg cIcon name="cilSync"></svg>
            </button>
          </div>
        </c-card-header>
        <c-card-body class="p-0">
          @if (loading()) {
            <div class="text-center py-5">
              <c-spinner color="primary" aria-label="Cargando"></c-spinner>
              <p class="text-body-secondary mt-3">Cargando {{ def.title }}…</p>
            </div>
          } @else if (error()) {
            <c-alert color="danger" class="m-3">
              <svg cIcon name="cilWarning" class="me-2"></svg>
              {{ error() }}
              <small class="d-block mt-1">Endpoint: <code>{{ lastUrl() }}</code></small>
            </c-alert>
          } @else if (rows().length === 0) {
            <div class="empty-state text-center py-5">
              <svg [cIcon]="def.icon" size="3xl" class="text-body-secondary"></svg>
              <h2 class="h5 mt-3">Sin datos</h2>
              <p class="text-body-secondary small">El endpoint retornó 0 items.</p>
            </div>
          } @else {
            <div class="table-responsive">
              <table class="table table-hover table-sm mb-0">
                <thead>
                  <tr>
                    @for (col of def.columns; track col.key) {
                      <th>{{ col.label }}</th>
                    }
                  </tr>
                </thead>
                <tbody>
                  @for (row of rows(); track $index) {
                    <tr>
                      @for (col of def.columns; track col.key) {
                        <td [class.font-monospace]="col.format === 'code'">
                          {{ formatCell(row, col) }}
                        </td>
                      }
                    </tr>
                  }
                </tbody>
              </table>
            </div>
          }
        </c-card-body>
      </c-card>
    } @else {
      <c-alert color="warning">
        <svg cIcon name="cilWarning" class="me-2"></svg>
        Mantenedor no encontrado en registry.
      </c-alert>
    }
  `,
  styles: [`
    .empty-state svg { opacity: 0.5; }
  `],
})
export class GenericMaintainerComponent implements OnInit {
  private readonly http = inject(HttpClient);
  private readonly route = inject(ActivatedRoute);

  @Input() path!: string;

  readonly rows = signal<any[]>([]);
  readonly loading = signal(false);
  readonly error = signal<string | null>(null);
  readonly projectFilter = signal('domain-services');
  readonly searchQuery = signal('');
  readonly lastUrl = signal('');

  readonly def = computed<MaintainerDef | undefined>(() =>
    MAINTAINERS.find(m => m.path === this.path),
  );

  /** Helper getter para usar `def` en el template sin invocar () cada vez. */
  get defValue(): MaintainerDef | undefined { return this.def(); }

  ngOnInit() {
    // Tomar el path de la URL si no se pasó como @Input.
    if (!this.path) {
      this.path = this.route.snapshot.url.map(s => s.path).join('/');
    }
    const def = this.def();
    if (def?.defaultParams?.['project_slug']) {
      this.projectFilter.set(def.defaultParams['project_slug']);
    }
    this.load();
  }

  load() {
    const def = this.def();
    if (!def) {
      this.error.set(`Maintainer "${this.path}" no está en el registry.`);
      return;
    }
    if (def.requireProject && !this.projectFilter().trim()) {
      this.error.set('project_slug es requerido');
      return;
    }
    this.loading.set(true);
    this.error.set(null);

    // Construir URL con query params
    const url = new URL(`${apiBase()}${def.endpoint}`, window.location.origin);
    if (def.requireProject) {
      url.searchParams.set('project_slug', this.projectFilter().trim());
    }
    if (def.hasSearch && this.searchQuery().trim()) {
      url.searchParams.set('q', this.searchQuery().trim());
    }
    this.lastUrl.set(url.pathname + url.search);

    this.http.get<{ data: any[] | null }>(url.pathname + url.search)
      .subscribe({
        next: r => {
          this.rows.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || `No se pudo cargar ${def.title}`);
          this.loading.set(false);
        },
      });
  }

  formatCell(row: any, col: ColumnDef): string {
    let val: any;
    try {
      val = col.value ? col.value(row) : row[col.key];
    } catch {
      val = '—';
    }
    if (val === null || val === undefined) {
      return col.nullable === false ? '' : '—';
    }
    if (col.format === 'date' && typeof val === 'string') {
      try { return new Date(val).toLocaleString(); } catch { return val; }
    }
    if (col.format === 'truncate' && typeof val === 'string') {
      return val.length > 40 ? val.slice(0, 40) + '…' : val;
    }
    if (col.format === 'json' && typeof val === 'object') {
      return JSON.stringify(val);
    }
    if (col.format === 'badge' && col.badgeColor) {
      // For badges we return just the value; the template could be enhanced to render a Badge.
      // For now we just return the value as plain text.
      return String(val);
    }
    if (typeof val === 'object') return JSON.stringify(val);
    return String(val);
  }
}
