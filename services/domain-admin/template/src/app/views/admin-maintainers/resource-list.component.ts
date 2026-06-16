import { Component, Input } from '@angular/core';
import { CommonModule, DatePipe, SlicePipe } from '@angular/common';
import { IconDirective } from '@coreui/icons-angular';

import {
  CardComponent, CardBodyComponent, CardHeaderComponent,
  SpinnerComponent, AlertComponent, BadgeComponent,
} from '@coreui/angular';

// HU-41.4: componente de tabla genérico para vistas de mantenedores.
// Renderiza una tabla con columnas configurables, estados loading/empty/error,
// y filtra/search simple opcional.

export interface ColumnDef<T = any> {
  key: string;
  label: string;
  /** Función que retorna el valor a mostrar. Si no se setea, usa row[key]. */
  value?: (row: T) => any;
  /** True si la celda debe renderizar como JSON pretty. */
  json?: boolean;
  /** Formato custom ('date' | 'badge' | 'truncate'). */
  format?: 'date' | 'truncate';
  /** True si la celda puede ser null (muestra '—'). */
  nullable?: boolean;
  /** Pipe width hint (1-12 de Bootstrap). */
  width?: number;
}

@Component({
  selector: 'app-resource-list',
  standalone: true,
  imports: [
    CommonModule, DatePipe, SlicePipe, IconDirective,
    CardComponent, CardBodyComponent, CardHeaderComponent,
    SpinnerComponent, AlertComponent, BadgeComponent,
  ],
  template: `
    <c-card>
      <c-card-header class="d-flex justify-content-between align-items-center">
        <div>
          <strong>{{ title }}</strong>
          @if (subtitle) {
            <small class="text-body-secondary ms-2">{{ subtitle }}</small>
          }
        </div>
        <div class="d-flex gap-2 align-items-center">
          @if (refresh) {
            <button cButton color="secondary" variant="outline" size="sm" (click)="refresh.emit()">
              <svg cIcon name="cilSync"></svg> Refrescar
            </button>
          }
          @if (rows().length > 0) {
            <span class="badge bg-primary">{{ rows().length }}</span>
          }
        </div>
      </c-card-header>
      <c-card-body class="p-0">
        @if (loading()) {
          <div class="text-center py-5">
            <c-spinner color="primary" aria-label="Cargando"></c-spinner>
            <p class="text-body-secondary mt-3">{{ loadingText }}</p>
          </div>
        } @else if (error()) {
          <c-alert color="danger" class="m-3">
            <svg cIcon name="cilWarning" class="me-2"></svg>{{ error() }}
          </c-alert>
        } @else if (rows().length === 0) {
          <div class="empty-state text-center py-5">
            <svg [cIcon]="emptyIcon" size="3xl" class="text-body-secondary"></svg>
            <h2 class="h5 mt-3">{{ emptyText }}</h2>
            @if (emptyHint) {
              <p class="text-body-secondary small mb-0">{{ emptyHint }}</p>
            }
          </div>
        } @else {
          <div class="table-responsive">
            <table class="table table-hover table-sm mb-0">
              <thead>
                <tr>
                  @for (col of columns; track col.key) {
                    <th [class.text-end]="col.key === '_actions'" [style.width.%]="col.width ? col.width * 8.33 : null">
                      {{ col.label }}
                    </th>
                  }
                </tr>
              </thead>
              <tbody>
                @for (row of rows(); track trackKey(row, $index)) {
                  <tr>
                    @for (col of columns; track col.key) {
                      <td [class.text-end]="col.key === '_actions'">
                        @if (col.key === '_actions') {
                          <ng-content [ngTemplateOutlet]="actionTpl" [ngTemplateOutletContext]="{$implicit: row}"></ng-content>
                        } @else {
                          {{ formatCell(row, col) }}
                        }
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
  `,
  styles: [`
    .empty-state svg { opacity: 0.5; }
  `],
})
export class ResourceListComponent<T = any> {
  @Input({ required: true }) title!: string;
  @Input() subtitle?: string;
  @Input({ required: true }) columns!: ColumnDef<T>[] | ColumnDef[];
  @Input() rows: any = () => [];
  @Input() loading: any = () => false;
  @Input() error: any = () => null;
  @Input() loadingText = 'Cargando…';
  @Input() emptyText = 'Sin datos';
  @Input() emptyHint?: string;
  @Input() emptyIcon: string = 'cilInbox';
  @Input() trackBy: (row: T, index: number) => any = (_, i) => i;
  @Input() refresh?: { emit: () => void };

  readonly actionTpl: any = null;

  trackKey(row: T, index: number): any {
    return this.trackBy(row, index);
  }

  formatCell(row: any, col: ColumnDef): string {
    const val = col.value ? col.value(row) : row[col.key];
    if (val === null || val === undefined) {
      return col.nullable === false ? '' : '—';
    }
    if (col.format === 'date' && typeof val === 'string') {
      return val;
    }
    if (col.format === 'truncate' && typeof val === 'string') {
      return val.length > 40 ? val.slice(0, 40) + '…' : val;
    }
    if (col.json && typeof val === 'object') {
      return JSON.stringify(val);
    }
    if (typeof val === 'object') {
      return JSON.stringify(val);
    }
    return String(val);
  }
}
