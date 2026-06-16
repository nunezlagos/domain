import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';

import { AlertComponent } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { ResourceListComponent, ColumnDef } from './resource-list.component';

interface Project {
  id: string;
  slug: string;
  name: string;
  description: string;
  organization_id: string;
  client_slug: string | null;
  repository_url: string | null;
  branch_default: string | null;
  created_at: string;
  updated_at: string;
}

// HU-41.4: Projects — listado de proyectos, repos, policies, scopes.
@Component({
  selector: 'app-projects',
  standalone: true,
  imports: [
    CommonModule, DatePipe, IconDirective,
    AlertComponent, ResourceListComponent,
  ],
  template: `
    <c-alert color="info" class="mb-3">
      <svg cIcon name="cilInfo" class="me-2"></svg>
      Los <strong>projects</strong> son unidades de scoping: knowledge, observations,
      policies, repos. Cada project pertenece a una org (y opcionalmente a un client).
    </c-alert>

    <app-resource-list
      [title]="'Proyectos registrados'"
      [columns]="columns"
      [rows]="projects"
      [loading]="loading"
      [error]="error"
      [emptyText]="'Sin projects'"
      [emptyHint]="'domain_project_create para registrar uno.'"
      emptyIcon="cilApps"
      (refresh)="load()">
    </app-resource-list>
  `,
})
export class ProjectsComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly projects = signal<Project[]>([]);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  readonly columns: ColumnDef<Project>[] = [
    { key: 'name', label: 'Nombre', nullable: false },
    { key: 'slug', label: 'Slug' },
    { key: 'client_slug', label: 'Cliente', value: r => r.client_slug ?? '—' },
    { key: 'repository_url', label: 'Repo', value: r => r.repository_url?.replace(/^https?:\/\//, ''), format: 'truncate' },
    { key: 'branch_default', label: 'Branch' },
    { key: 'updated_at', label: 'Actualizado', format: 'date' },
  ];

  ngOnInit() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.http.get<{ data: Project[] | null }>(`${apiBase()}/api/v1/projects`)
      .subscribe({
        next: r => {
          this.projects.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudieron cargar los projects');
          this.loading.set(false);
        },
      });
  }
}
