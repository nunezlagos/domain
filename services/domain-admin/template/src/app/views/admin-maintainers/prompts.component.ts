import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';
import { FormsModule } from '@angular/forms';

import { AlertComponent, ButtonDirective, FormControlDirective, InputGroupComponent, InputGroupTextDirective, BadgeComponent } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { ResourceListComponent, ColumnDef } from './resource-list.component';

interface CapturedPrompt {
  id: string;
  project_slug: string;
  content: string;
  char_count: number;
  estimated_tokens: number;
  client_kind: string;
  model: string;
  session_id: string | null;
  captured_at: string;
}

// HU-41.4: Captured Prompts — audit de todos los prompts crudos enviados al MCP.
// Sirve para debugging y para entender el uso real de la plataforma.
@Component({
  selector: 'app-prompts',
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
      Cada prompt que un user escribe se captura acá (sin PII). Útil para
      auditoría, debugging y métricas de uso real.
    </c-alert>

    <div class="mb-3 d-flex gap-2 align-items-center flex-wrap">
      <c-input-group style="max-width: 360px;">
        <span cInputGroupText>
          <svg cIcon name="cilApps"></svg>
        </span>
        <input
          cFormControl
          type="text"
          placeholder="project_slug (opcional)"
          [value]="projectFilter()"
          (input)="projectFilter.set($any($event.target).value)" />
      </c-input-group>
      <button cButton color="primary" (click)="load()">
        <svg cIcon name="cilList" class="me-1"></svg> Filtrar
      </button>
    </div>

    <app-resource-list
      [title]="'Captured Prompts'"
      [columns]="columns"
      [rows]="prompts"
      [loading]="loading"
      [error]="error"
      [emptyText]="'Sin prompts'"
      [emptyHint]="'Ningún prompt capturado todavía.'"
      emptyIcon="cilCopy"
      (refresh)="load()">
    </app-resource-list>
  `,
})
export class PromptsComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly prompts = signal<CapturedPrompt[]>([]);
  readonly projectFilter = signal('');
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  readonly columns: ColumnDef<CapturedPrompt>[] = [
    { key: 'content', label: 'Prompt', format: 'truncate' },
    { key: 'project_slug', label: 'Proyecto' },
    { key: 'model', label: 'Modelo' },
    { key: 'char_count', label: 'Chars' },
    { key: 'estimated_tokens', label: 'Tokens (est.)' },
    { key: 'client_kind', label: 'Cliente' },
    { key: 'captured_at', label: 'Capturado', format: 'date' },
  ];

  ngOnInit() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    const project = this.projectFilter().trim();
    const url = project
      ? `${apiBase()}/api/v1/captured-prompts?project_slug=${encodeURIComponent(project)}`
      : `${apiBase()}/api/v1/captured-prompts`;
    this.http.get<{ data: CapturedPrompt[] | null }>(url)
      .subscribe({
        next: r => {
          this.prompts.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudieron cargar los prompts');
          this.loading.set(false);
        },
      });
  }
}
