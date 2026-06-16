import { Component, computed, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { filter, map, startWith } from 'rxjs/operators';

import { IconDirective } from '@coreui/icons-angular';

import { AuthService } from '../../core/auth.service';
import { MAINTAINERS, SPECIAL_MAINTAINERS, MaintainerDef } from './maintainer-registry';

interface TabDef {
  path: string;
  title: string;
  icon: string;
  description: string;
  category?: string;
}

// HU-41.4: Mantenedores — vista padre. Tabs se generan desde el registry
// (data-driven). Agrupa por categoría con títulos visuales.
@Component({
  selector: 'app-maintainers',
  standalone: true,
  imports: [CommonModule, RouterOutlet, RouterLink, RouterLinkActive, IconDirective],
  template: `
    <section class="admin-maintainers">
      <header class="page-header mb-4">
        <h1 class="h3 mb-1">Mantenedores</h1>
        <p class="text-body-secondary mb-0">
          {{ allTabs.length }} vistas de inspección y mantenimiento de los recursos del MCP.
          Cada subtab consulta un endpoint REST del backend. Para agregar uno nuevo, editá <code>maintainer-registry.ts</code>.
        </p>
      </header>

      <nav class="maintainers-tabs mb-4">
        @for (cat of categories(); track cat.key) {
          <div class="cat-section mb-2">
            <small class="text-body-secondary text-uppercase fw-semibold">{{ cat.label }}</small>
            <ul class="nav nav-pills flex-wrap gap-1 mt-1">
              @for (tab of cat.tabs; track tab.path) {
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
          </div>
        }
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
    .cat-section { line-height: 1.1; }
  `],
})
export class MaintainersComponent {
  readonly auth = inject(AuthService);

  readonly allTabs: TabDef[] = [
    ...MAINTAINERS.map(m => ({ path: m.path, title: m.title, icon: m.icon, description: m.description, category: m.category })),
    ...SPECIAL_MAINTAINERS.map(m => ({ path: m.path, title: m.title, icon: m.icon, description: m.description, category: 'ops' })),
  ];

  readonly categories = computed(() => {
    const groups: Record<string, { key: string; label: string; tabs: TabDef[] }> = {
      core:          { key: 'core',          label: 'Identidad & Auth',     tabs: [] },
      resources:     { key: 'resources',     label: 'Recursos',              tabs: [] },
      observability: { key: 'observability', label: 'Observabilidad',        tabs: [] },
      system:        { key: 'system',        label: 'Sistema',               tabs: [] },
      sdd:           { key: 'sdd',           label: 'SDD / Specs',          tabs: [] },
      ops:           { key: 'ops',           label: 'Ops / Debug',          tabs: [] },
    };
    for (const tab of this.allTabs) {
      const cat = tab.category || 'ops';
      if (groups[cat]) groups[cat].tabs.push(tab);
    }
    return Object.values(groups).filter(g => g.tabs.length > 0);
  });
}
