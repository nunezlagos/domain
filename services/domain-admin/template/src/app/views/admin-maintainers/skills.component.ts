import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { FormsModule } from '@angular/forms';

import { AlertComponent, ButtonDirective, FormControlDirective, InputGroupComponent, InputGroupTextDirective } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { ResourceListComponent, ColumnDef } from './resource-list.component';

interface Skill {
  id: string;
  name: string;
  slug: string;
  description: string;
  skill_type: 'prompt' | 'code' | 'api' | 'mcp_tool';
  project_id: string | null;
  created_at: string;
  updated_at: string;
  version: number;
}

interface SkillSearchResult {
  id: string;
  name: string;
  description: string;
  skill_type: string;
  score?: number;
}

// HU-41.4: Mantenedor de skills — listar, buscar semánticamente, ejecutar.
@Component({
  selector: 'app-skills',
  standalone: true,
  imports: [
    CommonModule, DatePipe, FormsModule, IconDirective,
    AlertComponent, ButtonDirective,
    FormControlDirective, InputGroupComponent, InputGroupTextDirective,
    ResourceListComponent,
  ],
  template: `
    <c-alert color="info" class="mb-3">
      <svg cIcon name="cilInfo" class="me-2"></svg>
      Las <strong>skills</strong> son unidades reutilizables de lógica (prompts versionados, código, API calls).
      Se invocan con <code>domain_skill_execute</code> o desde agents/flows.
    </c-alert>

    <div class="mb-3 d-flex gap-2 align-items-center">
      <c-input-group>
        <span cInputGroupText>
          <svg cIcon name="cilSearch"></svg>
        </span>
        <input
          cFormControl
          type="search"
          placeholder="Búsqueda semántica de skills (ej: 'deploy backend')"
          [value]="searchQuery()"
          (input)="searchQuery.set($any($event.target).value)" />
      </c-input-group>
      <button cButton color="primary" (click)="doSearch()">
        <svg cIcon name="cilSearch" class="me-1"></svg> Buscar
      </button>
      <button cButton color="secondary" variant="outline" (click)="load()">
        <svg cIcon name="cilList" class="me-1"></svg> Listar todas
      </button>
    </div>

    @if (searchResults().length > 0) {
      <c-alert color="success" class="mb-2">
        <strong>{{ searchResults().length }}</strong> resultados para "{{ lastQuery() }}"
      </c-alert>
    }

    <app-resource-list
      [title]="searchResults().length > 0 ? 'Resultados de búsqueda' : 'Skills registradas'"
      [columns]="columns"
      [rows]="searchResults().length > 0 ? searchResults : skills"
      [loading]="loading"
      [error]="error"
      [emptyText]="searchResults().length > 0 ? 'Sin resultados' : 'Sin skills'"
      [emptyHint]="'domain_skill_search retorna 0 matches.'"
      emptyIcon="cilBolt">
    </app-resource-list>
  `,
})
export class SkillsComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly skills = signal<Skill[]>([]);
  readonly searchResults = signal<SkillSearchResult[]>([]);
  readonly searchQuery = signal('');
  readonly lastQuery = signal('');
  readonly loading = signal(false);
  readonly error = signal<string | null>(null);

  readonly columns: ColumnDef<Skill>[] = [
    { key: 'name', label: 'Nombre', nullable: false },
    { key: 'slug', label: 'Slug' },
    { key: 'skill_type', label: 'Tipo' },
    { key: 'description', label: 'Descripción', format: 'truncate' },
    { key: 'version', label: 'Versión' },
    { key: 'updated_at', label: 'Actualizado', format: 'date' },
  ];

  ngOnInit() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.searchResults.set([]);
    this.http.get<{ data: Skill[] | null }>(`${apiBase()}/api/v1/skills`)
      .subscribe({
        next: r => {
          this.skills.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudieron cargar las skills');
          this.loading.set(false);
        },
      });
  }

  doSearch() {
    const q = this.searchQuery().trim();
    if (!q) {
      this.searchResults.set([]);
      return;
    }
    this.loading.set(true);
    this.error.set(null);
    this.lastQuery.set(q);
    this.http.post<{ data: SkillSearchResult[] | null }>(`${apiBase()}/api/v1/skills/search`, { query: q, limit: 20 })
      .subscribe({
        next: r => {
          this.searchResults.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'Búsqueda falló');
          this.loading.set(false);
        },
      });
  }
}
