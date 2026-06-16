import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { FormsModule } from '@angular/forms';

import { AlertComponent, ButtonDirective, FormControlDirective, InputGroupComponent, InputGroupTextDirective, BadgeComponent } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { ResourceListComponent, ColumnDef } from './resource-list.component';

interface Observation {
  id: string;
  project_slug: string;
  observation_type: 'note' | 'decision' | 'bug' | 'pattern' | 'discovery';
  content: string;
  tags: string[];
  created_at: string;
  updated_at: string;
}

// HU-41.4: Mantenedor de observations (memoria persistente) — listar por proyecto, buscar.
@Component({
  selector: 'app-observations',
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
      Las <strong>observations</strong> son memoria persistente cross-session (mem_save / mem_search).
      Búsqueda híbrida: vector + BM25 + RRF.
    </c-alert>

    <div class="mb-3 d-flex gap-2 align-items-center flex-wrap">
      <c-input-group style="max-width: 360px;">
        <span cInputGroupText>
          <svg cIcon name="cilApps"></svg>
        </span>
        <input
          cFormControl
          type="text"
          placeholder="project_slug (req.)"
          [value]="projectFilter()"
          (input)="projectFilter.set($any($event.target).value)" />
      </c-input-group>
      <button cButton color="primary" (click)="load()">
        <svg cIcon name="cilList" class="me-1"></svg> Listar
      </button>
      <button cButton color="secondary" variant="outline" (click)="doSearch()">
        <svg cIcon name="cilSearch" class="me-1"></svg> Buscar
      </button>
    </div>

    <app-resource-list
      [title]="searchMode() ? 'Resultados de búsqueda' : 'Observations'"
      [columns]="columns"
      [rows]="results"
      [loading]="loading"
      [error]="error"
      [emptyText]="searchMode() ? 'Sin matches' : 'Sin observations'"
      [emptyHint]="searchMode() ? 'Probá otra query.' : 'domain_mem_save para registrar.'"
      emptyIcon="cilList"
      (refresh)="load()">
    </app-resource-list>
  `,
})
export class ObservationsComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly observations = signal<Observation[]>([]);
  readonly results = signal<Observation[]>([]);
  readonly searchMode = signal(false);
  readonly projectFilter = signal('domain-services');
  readonly loading = signal(false);
  readonly error = signal<string | null>(null);

  readonly columns: ColumnDef<Observation>[] = [
    { key: 'observation_type', label: 'Tipo' },
    { key: 'project_slug', label: 'Proyecto' },
    { key: 'content', label: 'Contenido', format: 'truncate' },
    { key: 'tags', label: 'Tags', value: r => (r.tags || []).join(', '), nullable: true },
    { key: 'updated_at', label: 'Actualizado', format: 'date' },
  ];

  ngOnInit() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.searchMode.set(false);
    const project = this.projectFilter().trim();
    const url = project
      ? `${apiBase()}/api/v1/observations?project_slug=${encodeURIComponent(project)}`
      : `${apiBase()}/api/v1/observations`;
    this.http.get<{ data: Observation[] | null }>(url)
      .subscribe({
        next: r => {
          this.observations.set(r?.data ?? []);
          this.results.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudieron cargar las observations');
          this.loading.set(false);
        },
      });
  }

  doSearch() {
    const q = this.projectFilter().trim();
    if (!q) {
      this.load();
      return;
    }
    this.loading.set(true);
    this.error.set(null);
    this.searchMode.set(true);
    this.http.get<{ data: Observation[] | null }>(`${apiBase()}/api/v1/search?q=${encodeURIComponent(q)}`)
      .subscribe({
        next: r => {
          this.results.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'Búsqueda falló');
          this.loading.set(false);
        },
      });
  }
}
