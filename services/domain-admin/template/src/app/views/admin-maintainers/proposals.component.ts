import { Component, inject, signal, OnInit } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { HttpClient } from '@angular/common/http';

import { AlertComponent, BadgeComponent, ButtonDirective } from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';
import { ResourceListComponent, ColumnDef } from './resource-list.component';

interface Proposal {
  id: string;
  kind: 'policy' | 'skill';
  slug: string;
  name: string;
  project_slug: string | null;
  proposed: boolean;
  source: string;
  rationale: string;
  created_at: string;
}

// HU-41.4: Proposals — accept/reject de skills y policies propuestas por LLMs.
// Las propuestas viven en el dominio y requieren review humano antes de activarse.
@Component({
  selector: 'app-proposals',
  standalone: true,
  imports: [
    CommonModule, DatePipe, IconDirective,
    AlertComponent, BadgeComponent, ButtonDirective,
    ResourceListComponent,
  ],
  template: `
    <c-alert color="warning" class="mb-3">
      <svg cIcon name="cilWarning" class="me-2"></svg>
      Proposals son skills/policies generadas por LLMs. <strong>NUNCA se aceptan automáticamente.</strong>
      Requieren review humano (vos) antes de quedar activas.
    </c-alert>

    <app-resource-list
      [title]="'Proposals pendientes'"
      [subtitle]="'(skills + policies generadas por IA)'"
      [columns]="columns"
      [rows]="proposals"
      [loading]="loading"
      [error]="error"
      [emptyText]="'Sin proposals pendientes'"
      [emptyHint]="'Todas las proposals ya fueron revisadas.'"
      emptyIcon="cilCheckCircle"
      (refresh)="load()">
    </app-resource-list>
  `,
})
export class ProposalsComponent implements OnInit {
  private readonly http = inject(HttpClient);

  readonly proposals = signal<Proposal[]>([]);
  readonly loading = signal(true);
  readonly error = signal<string | null>(null);

  readonly columns: ColumnDef<Proposal>[] = [
    { key: 'kind', label: 'Tipo' },
    { key: 'name', label: 'Nombre', nullable: false },
    { key: 'slug', label: 'Slug' },
    { key: 'project_slug', label: 'Proyecto', value: r => r.project_slug ?? 'global' },
    { key: 'source', label: 'Source' },
    { key: 'rationale', label: 'Rationale', format: 'truncate' },
    { key: 'created_at', label: 'Creado', format: 'date' },
  ];

  ngOnInit() { this.load(); }

  load() {
    this.loading.set(true);
    this.error.set(null);
    this.http.get<{ data: Proposal[] | null }>(`${apiBase()}/api/v1/proposals`)
      .subscribe({
        next: r => {
          this.proposals.set(r?.data ?? []);
          this.loading.set(false);
        },
        error: err => {
          this.error.set(err?.error?.error?.message || 'No se pudieron cargar las proposals');
          this.loading.set(false);
        },
      });
  }
}
