import { Component, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { filter, map, startWith } from 'rxjs/operators';

import { IconDirective } from '@coreui/icons-angular';

import { AuthService } from '../../core/auth.service';

interface TabDef {
  path: string;
  title: string;
  icon: string;
  description: string;
}

// HU-41.4: Mantenedores — vista padre. Muestra tab pills horizontales
// con los 12 subtabs de mantenimiento. Mantiene el usuario en el tab
// actual al recargar (vía router state).
@Component({
  selector: 'app-maintainers',
  standalone: true,
  imports: [CommonModule, RouterOutlet, RouterLink, RouterLinkActive, IconDirective],
  template: `
    <section class="admin-maintainers">
      <header class="page-header mb-4">
        <h1 class="h3 mb-1">Mantenedores</h1>
        <p class="text-body-secondary mb-0">
          Inspección y mantenimiento de los recursos internos del MCP: API keys, skills, agents, flows, crons, observations, audit log, proposals, projects, knowledge y system.
        </p>
      </header>

      <nav class="maintainers-tabs mb-4">
        <ul class="nav nav-pills flex-wrap gap-1">
          @for (tab of tabs; track tab.path) {
            <li class="nav-item">
              <a
                class="nav-link d-flex align-items-center gap-2"
                [routerLink]="['/admin/maintainers', tab.path]"
                routerLinkActive="active"
                [attr.title]="tab.description">
                <svg [cIcon]="tab.icon"></svg>
                <span class="d-none d-md-inline">{{ tab.title }}</span>
              </a>
            </li>
          }
        </ul>
      </nav>

      <div class="maintainers-tab-content">
        <router-outlet />
      </div>
    </section>
  `,
  styles: [`
    .maintainers-tabs .nav-link {
      color: var(--cui-body-color);
      font-size: 0.875rem;
    }
    .maintainers-tabs .nav-link.active {
      background-color: var(--cui-primary);
      color: #fff;
    }
    .maintainers-tabs .nav-link svg {
      width: 1rem;
      height: 1rem;
    }
  `],
})
export class MaintainersComponent {
  readonly auth = inject(AuthService);

  readonly tabs: TabDef[] = [
    { path: 'api-keys',     title: 'API Keys',        icon: 'cilFingerprint',          description: 'API keys emitidas, revocación y rotación.' },
    { path: 'skills',       title: 'Skills',          icon: 'cilBolt',                 description: 'Skills globales y del proyecto.' },
    { path: 'agents',       title: 'Agents',          icon: 'cilTerminal',             description: 'Agents registrados y ejecuciones.' },
    { path: 'flows',        title: 'Flows',           icon: 'cilShareAll',             description: 'Flows DAG, runs y export/import.' },
    { path: 'crons',        title: 'Crons',           icon: 'cilClock',                description: 'Schedules recurrentes.' },
    { path: 'observations', title: 'Observations',    icon: 'cilList',                 description: 'Memoria persistente.' },
    { path: 'prompts',      title: 'Prompts',         icon: 'cilCopy',                 description: 'Captured prompts con metadata.' },
    { path: 'audit',        title: 'Audit Log',       icon: 'cilHistory',              description: 'Acciones registradas.' },
    { path: 'proposals',    title: 'Proposals',       icon: 'cilCheckCircle',          description: 'Skills/policies propuestas.' },
    { path: 'projects',     title: 'Projects',        icon: 'cilApps',                 description: 'Proyectos registrados.' },
    { path: 'knowledge',    title: 'Knowledge',       icon: 'cilBook',                 description: 'Documentos chunkeados para RAG.' },
    { path: 'system',       title: 'System',          icon: 'cilSpeedometer',          description: 'Health y configuración runtime.' },
  ];
}
