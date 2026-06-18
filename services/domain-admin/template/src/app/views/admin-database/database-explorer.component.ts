import { Component, inject, signal, computed, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { FormsModule } from '@angular/forms';

import {
  CardComponent, CardBodyComponent, CardHeaderComponent,
  BadgeComponent, ButtonDirective, FormControlDirective,
  InputGroupComponent, InputGroupTextDirective,
  AlertComponent, SpinnerComponent,
} from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';

interface ColumnInfo {
  name: string;
  type: string;
  is_nullable: boolean;
  default_value?: string;
  is_primary_key: boolean;
  is_foreign_key: boolean;
}

interface IndexInfo {
  name: string;
  columns: string;
  is_unique: boolean;
  is_primary: boolean;
}

interface FKInfo {
  constraint: string;
  columns: string;
  references: string;
}

interface TableInfo {
  name: string;
  schema: string;
  row_count: number;
  size_bytes: number;
  has_created_at: boolean;
  has_updated_at: boolean;
  has_status: boolean;
  columns: ColumnInfo[];
  indexes: IndexInfo[];
  foreign_keys: FKInfo[];
  category: string;
}

interface SchemaResponse {
  data: {
    tables: TableInfo[];
    total_count: number;
  };
}

const CATEGORY_META: Record<string, { label: string; color: string; icon: string }> = {
  core:          { label: 'Identidad & Auth',     color: 'primary',   icon: 'cilUser' },
  resources:     { label: 'Recursos',              color: 'info',      icon: 'cilApps' },
  observability: { label: 'Observabilidad',        color: 'warning',   icon: 'cilList' },
  system:        { label: 'Sistema',               color: 'success',   icon: 'cilStorage' },
  sdd:           { label: 'SDD / Specs',          color: 'secondary', icon: 'cilFile' },
  other:         { label: 'Otros',                 color: 'dark',      icon: 'cilSettings' },
};

// HU-41.4: DB Explorer — vista del schema completo con tablas, columnas, FKs, índices, row counts.
// Alimentado por GET /api/v1/admin/db-schema (admin only).
@Component({
  selector: 'app-database-explorer',
  standalone: true,
  imports: [
    CommonModule, DatePipe, FormsModule, IconDirective,
    CardComponent, CardBodyComponent, CardHeaderComponent,
    BadgeComponent, ButtonDirective, FormControlDirective,
    InputGroupComponent, InputGroupTextDirective,
    AlertComponent, SpinnerComponent,
  ],
  template: `
    <section class="db-explorer">
      <header class="page-header mb-3">
        <h1 class="h3 mb-1">Base de datos</h1>
        <p class="text-body-secondary mb-0">
          Schema completo del dominio: {{ tables().length }} tablas operativas,
          con columnas, foreign keys, índices y row counts.
          Buscá una tabla o filtrá por categoría.
        </p>
      </header>

      <!-- Search + category filters -->
      <div class="d-flex gap-2 mb-3 align-items-center flex-wrap">
        <c-input-group style="max-width: 360px;">
          <span cInputGroupText>
            <svg cIcon name="cilSearch"></svg>
          </span>
          <input
            cFormControl
            type="search"
            placeholder="Buscar tabla o columna..."
            [value]="query()"
            (input)="query.set($any($event.target).value)" />
        </c-input-group>
        <button
          cButton size="sm" color="secondary" variant="outline"
          (click)="reload()">
          <svg cIcon name="cilSync" class="me-1"></svg> Refrescar
        </button>
        <span class="text-body-secondary small ms-auto">
          {{ filteredTables().length }} de {{ tables().length }} tablas
        </span>
      </div>

      @if (loading()) {
        <div class="text-center py-5">
          <c-spinner color="primary"></c-spinner>
          <p class="text-body-secondary mt-3">Cargando schema...</p>
        </div>
      } @else if (error()) {
        <c-alert color="danger">
          <svg cIcon name="cilWarning" class="me-2"></svg>{{ error() }}
        </c-alert>
      } @else {
        <!-- Tables grouped by category -->
        @for (cat of groupedTables(); track cat.key) {
          <div class="mb-4">
            <h2 class="h5 mb-2">
              <svg [cIcon]="cat.meta.icon" class="me-2"></svg>
              {{ cat.meta.label }}
              <span class="badge bg-secondary ms-2">{{ cat.tables.length }}</span>
            </h2>
            <div class="row g-3">
              @for (tbl of cat.tables; track tbl.name) {
                <div class="col-12">
                  <c-card>
                    <c-card-header
                      class="d-flex justify-content-between align-items-center"
                      role="button"
                      (click)="toggleTable(tbl.name)"
                      style="cursor: pointer;">
                      <div>
                        <svg cIcon name="cilGrid" class="me-2"></svg>
                        <code class="fs-6">{{ tbl.name }}</code>
                        @if (isExpanded(tbl.name)) {
                          <svg cIcon name="cilChevronTop" class="ms-2"></svg>
                        } @else {
                          <svg cIcon name="cilChevronBottom" class="ms-2"></svg>
                        }
                      </div>
                      <div class="d-flex gap-2 align-items-center">
                        <c-badge color="info">{{ tbl.row_count }} rows</c-badge>
                        @if (tbl.has_created_at) {
                          <c-badge color="success" title="Tiene created_at">+c</c-badge>
                        } @else {
                          <c-badge color="danger" title="Sin created_at">-c</c-badge>
                        }
                        @if (tbl.has_updated_at) {
                          <c-badge color="success" title="Tiene updated_at">+u</c-badge>
                        } @else {
                          <c-badge color="danger" title="Sin updated_at">-u</c-badge>
                        }
                        @if (tbl.has_status) {
                          <c-badge color="success" title="Tiene status">+s</c-badge>
                        } @else {
                          <c-badge color="warning" title="Sin status">-s</c-badge>
                        }
                      </div>
                    </c-card-header>
                    @if (isExpanded(tbl.name)) {
                      <c-card-body>
                        <div class="row g-3">
                          <!-- Columns -->
                          <div class="col-md-6">
                            <h3 class="h6 text-uppercase text-body-secondary">
                              <svg cIcon name="cilColumns" class="me-1"></svg>
                              Columnas ({{ tbl.columns.length }})
                            </h3>
                            <table class="table table-sm table-striped mb-0">
                              <thead>
                                <tr>
                                  <th>Nombre</th>
                                  <th>Tipo</th>
                                  <th>Null</th>
                                </tr>
                              </thead>
                              <tbody>
                                @for (col of tbl.columns; track col.name) {
                                  <tr>
                                    <td>
                                      <code>{{ col.name }}</code>
                                      @if (col.is_primary_key) {
                                        <c-badge color="warning" class="ms-1">PK</c-badge>
                                      }
                                      @if (col.is_foreign_key) {
                                        <c-badge color="info" class="ms-1">FK</c-badge>
                                      }
                                    </td>
                                    <td><small class="text-body-secondary">{{ col.type }}</small></td>
                                    <td>
                                      @if (col.is_nullable) {
                                        <span class="text-body-secondary small">YES</span>
                                      } @else {
                                        <span class="text-body small">NO</span>
                                      }
                                    </td>
                                  </tr>
                                }
                              </tbody>
                            </table>
                          </div>

                          <!-- Indexes + FKs -->
                          <div class="col-md-6">
                            @if (tbl.foreign_keys.length > 0) {
                              <h3 class="h6 text-uppercase text-body-secondary">
                                <svg cIcon name="cilLink" class="me-1"></svg>
                                Foreign keys ({{ tbl.foreign_keys.length }})
                              </h3>
                              <ul class="list-unstyled small">
                                @for (fk of tbl.foreign_keys; track fk.constraint) {
                                  <li class="mb-1">
                                    <code class="text-body-secondary">{{ fk.columns }}</code>
                                    <span class="mx-1">→</span>
                                    <code>{{ fk.references }}</code>
                                  </li>
                                }
                              </ul>
                            }
                            @if (tbl.indexes.length > 0) {
                              <h3 class="h6 text-uppercase text-body-secondary mt-3">
                                <svg cIcon name="cilLayers" class="me-1"></svg>
                                Índices ({{ tbl.indexes.length }})
                              </h3>
                              <ul class="list-unstyled small">
                                @for (idx of tbl.indexes; track idx.name) {
                                  <li class="mb-1">
                                    <code>{{ idx.name }}</code>
                                    <small class="text-body-secondary ms-1">({{ idx.columns }})</small>
                                    @if (idx.is_unique) {
                                      <c-badge color="info" class="ms-1">UNIQUE</c-badge>
                                    }
                                  </li>
                                }
                              </ul>
                            }
                          </div>
                        </div>
                      </c-card-body>
                    }
                  </c-card>
                </div>
              }
            </div>
          </div>
        }
      }
    </section>
  `,
  styles: [`
    code { font-size: 0.85em; }
  `],
})
export class DatabaseExplorerComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly tables = signal<TableInfo[]>([]);
  readonly query = signal('');
  readonly loading = signal(false);
  readonly error = signal<string | null>(null);
  readonly expanded = signal<Set<string>>(new Set());

  readonly filteredTables = computed<TableInfo[]>(() => {
    const q = this.query().trim().toLowerCase();
    if (!q) return this.tables();
    return this.tables().filter(t => {
      if (t.name.toLowerCase().includes(q)) return true;
      // también buscar dentro de las columnas
      return t.columns.some(c => c.name.toLowerCase().includes(q));
    });
  });

  readonly groupedTables = computed(() => {
    const grouped: Record<string, { key: string; meta: typeof CATEGORY_META[string]; tables: TableInfo[] }> = {};
    for (const [key, meta] of Object.entries(CATEGORY_META)) {
      grouped[key] = { key, meta, tables: [] };
    }
    for (const tbl of this.filteredTables()) {
      if (grouped[tbl.category]) grouped[tbl.category].tables.push(tbl);
    }
    return Object.values(grouped).filter(g => g.tables.length > 0);
  });

  ngOnInit() { this.load(); }

  reload() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.http.get<SchemaResponse>(`${apiBase()}/api/v1/admin/db-schema`)
      .subscribe({
        next: r => {
          this.tables.set(r?.data?.tables ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudo cargar el schema');
          this.loading.set(false);
        },
      });
  }

  toggleTable(name: string) {
    const next = new Set(this.expanded());
    if (next.has(name)) next.delete(name); else next.add(name);
    this.expanded.set(next);
  }

  isExpanded(name: string): boolean {
    return this.expanded().has(name);
  }
}
