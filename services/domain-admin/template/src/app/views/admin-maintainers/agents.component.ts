import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';

import { AlertComponent } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { ResourceListComponent, ColumnDef } from './resource-list.component';

interface Agent {
  id: string;
  name: string;
  slug: string;
  description: string;
  provider: string;
  model: string;
  system_prompt: string;
  project_id: string | null;
  created_at: string;
  updated_at: string;
  version: number;
}

// HU-41.4: Mantenedor de agents — listar, ver versiones, ejecutar.
@Component({
  selector: 'app-agents',
  standalone: true,
  imports: [
    CommonModule, DatePipe, IconDirective,
    AlertComponent, ResourceListComponent,
  ],
  template: `
    <c-alert color="info" class="mb-3">
      <svg cIcon name="cilInfo" class="me-2"></svg>
      Los <strong>agents</strong> combinan LLM + skills + system prompt.
      Se ejecutan con <code>domain_agent_run</code> o vía flows.
    </c-alert>

    <app-resource-list
      [title]="'Agents registrados'"
      [columns]="columns"
      [rows]="agents"
      [loading]="loading"
      [error]="error"
      [emptyText]="'Sin agents'"
      [emptyHint]="'domain_agent_create para registrar uno nuevo.'"
      emptyIcon="cilTerminal"
      (refresh)="load()">
    </app-resource-list>
  `,
})
export class AgentsComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly agents = signal<Agent[]>([]);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  readonly columns: ColumnDef<Agent>[] = [
    { key: 'name', label: 'Nombre', nullable: false },
    { key: 'slug', label: 'Slug' },
    { key: 'provider', label: 'Provider' },
    { key: 'model', label: 'Modelo' },
    { key: 'description', label: 'Descripción', format: 'truncate' },
    { key: 'version', label: 'Ver.' },
    { key: 'updated_at', label: 'Actualizado', format: 'date' },
  ];

  ngOnInit() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.http.get<{ data: Agent[] | null }>(`${apiBase()}/api/v1/agents`)
      .subscribe({
        next: r => {
          this.agents.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudieron cargar los agents');
          this.loading.set(false);
        },
      });
  }
}
