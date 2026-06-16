import { Component, inject, signal, computed, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { FormsModule } from '@angular/forms';

import { AlertComponent, ButtonDirective, FormControlDirective, InputGroupComponent, InputGroupTextDirective, BadgeComponent } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { ResourceListComponent, ColumnDef } from './resource-list.component';

interface KnowledgeDoc {
  id: string;
  title: string;
  project_slug: string | null;
  source: string;
  source_url: string | null;
  tags: string[];
  body_preview: string;
  chunk_count: number;
  created_at: string;
  updated_at: string;
}

interface KnowledgeSearchResult {
  doc_id: string;
  title: string;
  chunk_preview: string;
  score: number;
}

// HU-41.4: Knowledge — documentos chunkeados para RAG, búsqueda semántica.
@Component({
  selector: 'app-knowledge',
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
      Los <strong>knowledge docs</strong> son documentos largos chunkeados con embeddings
      para RAG. Búsqueda por similitud + BM25.
    </c-alert>

    <div class="mb-3 d-flex gap-2 align-items-center flex-wrap">
      <c-input-group style="max-width: 200px;">
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
      <c-input-group style="max-width: 360px;">
        <span cInputGroupText>
          <svg cIcon name="cilSearch"></svg>
        </span>
        <input
          cFormControl
          type="search"
          placeholder="Buscar (semantic + BM25)"
          [value]="searchQuery()"
          (input)="searchQuery.set($any($event.target).value)" />
      </c-input-group>
      <button cButton color="primary" (click)="doSearch()">Buscar</button>
      <button cButton color="secondary" variant="outline" (click)="load()">Listar</button>
    </div>

    <app-resource-list
      [title]="searchMode() ? 'Resultados' : 'Knowledge docs'"
      [columns]="activeColumns()"
      [rows]="searchMode() ? searchResults : docs"
      [loading]="loading"
      [error]="error"
      [emptyText]="searchMode() ? 'Sin matches' : 'Sin docs'"
      [emptyHint]="searchMode() ? 'Probá otra query.' : 'domain_knowledge_save para registrar.'"
      emptyIcon="cilBook"
      (refresh)="load()">
    </app-resource-list>
  `,
})
export class KnowledgeComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly docs = signal<KnowledgeDoc[]>([]);
  readonly searchResults = signal<KnowledgeSearchResult[]>([]);
  readonly searchQuery = signal('');
  readonly projectFilter = signal('domain-services');
  readonly searchMode = signal(false);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  readonly docColumns: ColumnDef<KnowledgeDoc>[] = [
    { key: 'title', label: 'Título', nullable: false },
    { key: 'project_slug', label: 'Proyecto', value: r => r.project_slug ?? '—' },
    { key: 'source', label: 'Source' },
    { key: 'chunk_count', label: 'Chunks' },
    { key: 'tags', label: 'Tags', value: r => (r.tags || []).join(', '), nullable: true },
    { key: 'updated_at', label: 'Actualizado', format: 'date' },
  ];

  readonly searchColumns: ColumnDef<KnowledgeSearchResult>[] = [
    { key: 'title', label: 'Doc' },
    { key: 'chunk_preview', label: 'Match', format: 'truncate' },
    { key: 'score', label: 'Score', value: r => r.score?.toFixed(3) },
  ];

  readonly activeColumns = computed(() =>
    this.searchMode() ? this.searchColumns : this.docColumns,
  );

  ngOnInit() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.searchMode.set(false);
    const project = this.projectFilter().trim();
    if (!project) {
      this.error.set('project_slug es requerido');
      this.loading.set(false);
      return;
    }
    this.http.get<{ data: KnowledgeDoc[] | null }>(`${apiBase()}/api/v1/knowledge?project_slug=${encodeURIComponent(project)}`)
      .subscribe({
        next: r => {
          this.docs.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudieron cargar los docs');
          this.loading.set(false);
        },
      });
  }

  doSearch() {
    const project = this.projectFilter().trim();
    const q = this.searchQuery().trim();
    if (!project) {
      this.error.set('project_slug es requerido');
      return;
    }
    if (!q) {
      this.load();
      return;
    }
    this.loading.set(true);
    this.error.set(null);
    this.searchMode.set(true);
    this.http.get<{ data: KnowledgeSearchResult[] | null }>(`${apiBase()}/api/v1/knowledge/search?project_slug=${encodeURIComponent(project)}&q=${encodeURIComponent(q)}&limit=20`)
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
