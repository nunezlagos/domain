import { Component, computed, inject, signal, ElementRef, ViewChild, HostListener } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { toSignal } from '@angular/core/rxjs-interop';
import { filter, map, startWith } from 'rxjs/operators';

import {
  FormControlDirective, InputGroupComponent, InputGroupTextDirective, ButtonDirective,
  AlertComponent,
} from '@coreui/angular';
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

const CATEGORY_LABELS: Record<string, string> = {
  core: 'Identidad & Auth',
  resources: 'Recursos',
  observability: 'Observabilidad',
  system: 'Sistema',
  sdd: 'SDD / Specs',
  ops: 'Ops / Debug',
};

// HU-41.4: Mantenedores — vista padre. Tabs se generan desde el registry
// (data-driven). Agrupa por categoría con títulos visuales.
// Search input global arriba filtra tabs en tiempo real.
@Component({
  selector: 'app-maintainers',
  standalone: true,
  imports: [
    CommonModule, FormsModule, RouterOutlet, RouterLink, RouterLinkActive, IconDirective,
    FormControlDirective, InputGroupComponent, InputGroupTextDirective, ButtonDirective,
    AlertComponent,
  ],
  template: `
    <section class="admin-maintainers">
      <header class="page-header mb-3">
        <h1 class="h3 mb-1">Mantenedores</h1>
        <p class="text-body-secondary mb-0">
          {{ allTabs.length }} vistas de inspección y mantenimiento de los recursos del MCP.
        </p>
      </header>

      <!-- Search global de mantenedores (icon a la derecha, sin chips) -->
      <div class="maintainers-search mb-3 position-relative">
        <c-input-group>
          <input
            #searchInput
            cFormControl
            type="search"
            role="combobox"
            aria-autocomplete="list"
            [attr.aria-expanded]="showSearchSuggestions()"
            [attr.aria-activedescendant]="activeSearchId()"
            placeholder="Buscar mantenedor (ej: 'agents', 'audit', 'cost') — ⌘K para enfocar"
            [value]="searchQuery()"
            (input)="onSearchChange($any($event.target).value)"
            (focus)="showSearchSuggestions.set(true)"
            (keydown)="onSearchKeydown($event)"
            autocomplete="off" />
          @if (searchQuery()) {
            <button
              cButton
              color="secondary"
              variant="outline"
              type="button"
              (click)="clearSearch()"
              aria-label="Limpiar búsqueda">
              <svg cIcon name="cilX"></svg>
            </button>
          }
          <span cInputGroupText>
            <svg cIcon name="cilSearch"></svg>
          </span>
        </c-input-group>

        <!-- Suggestions dropdown (formato: Categoría → descripción) -->
        @if (showSearchSuggestions() && searchQuery().length > 0) {
          <ul class="search-suggestions list-unstyled shadow-sm" role="listbox">
            @for (tab of filteredTabs(); track tab.path; let i = $index) {
              <li
                [id]="searchSuggestionId(i)"
                role="option"
                [class.active]="i === activeSearchIndex()"
                (mouseenter)="activeSearchIndex.set(i)"
                (click)="navigateTo(tab)">
                <div class="d-flex align-items-center gap-2">
                  <svg [cIcon]="tab.icon" class="flex-shrink-0"></svg>
                  <div class="flex-grow-1">
                    <strong>{{ tab.title }}</strong>
                    <small class="text-body-secondary d-block">
                      <span class="text-uppercase fw-semibold">{{ categoryLabel(tab.category) }}</span>
                      <span class="mx-1">→</span>
                      <span>{{ tab.description }}</span>
                    </small>
                  </div>
                </div>
              </li>
            } @empty {
              <li class="empty text-body-secondary text-center py-3">
                Sin mantenedores que matcheen "{{ searchQuery() }}"
              </li>
            }
          </ul>
        }
      </div>

      <nav class="maintainers-tabs mb-4">
        <!-- Solo el buscador. Sin pills/chips de navegación. -->
        @if (searchQuery().length > 0 && filteredTabs().length === 0) {
          <c-alert color="info">
            <svg cIcon name="cilInfo" class="me-2"></svg>
            Sin mantenedores que matcheen "<strong>{{ searchQuery() }}</strong>". Probá otro término o
            <a href="javascript:" (click)="clearSearch()">limpiá la búsqueda</a>.
          </c-alert>
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

    .search-suggestions {
      position: absolute;
      top: 100%;
      left: 0;
      right: 0;
      z-index: 1000;
      max-height: 360px;
      overflow-y: auto;
      background: var(--cui-body-bg);
      border: 1px solid var(--cui-border-color);
      border-radius: var(--cui-border-radius);
      margin-top: 4px;
      padding: 4px 0;
    }

    .search-suggestions li {
      padding: 10px 14px;
      cursor: pointer;
      border-left: 3px solid transparent;
    }

    .search-suggestions li:hover,
    .search-suggestions li.active {
      background: var(--cui-tertiary-bg);
      border-left-color: var(--cui-primary);
    }

    .search-suggestions li.empty {
      cursor: default;
    }

    .search-suggestions li.empty:hover {
      background: transparent;
      border-left-color: transparent;
    }

    .highlight-search {
      background: color-mix(in srgb, var(--cui-warning) 30%, transparent);
      border-radius: 2px;
      padding: 0 2px;
    }
  `],
})
export class MaintainersComponent {
  private readonly host = inject(ElementRef<HTMLElement>);

  @ViewChild('searchInput') searchInputRef?: ElementRef<HTMLInputElement>;

  readonly allTabs: TabDef[] = [
    ...MAINTAINERS.map(m => ({ path: m.path, title: m.title, icon: m.icon, description: m.description, category: m.category })),
    ...SPECIAL_MAINTAINERS.map(m => ({ path: m.path, title: m.title, icon: m.icon, description: m.description, category: 'ops' })),
  ];

  // === Search state ===
  readonly searchQuery = signal('');
  readonly showSearchSuggestions = signal(false);
  readonly activeSearchIndex = signal(-1);

  readonly filteredTabs = computed(() => {
    const q = this.searchQuery().trim().toLowerCase();
    if (!q) return [];
    return this.allTabs.filter(t =>
      t.title.toLowerCase().includes(q) ||
      t.path.toLowerCase().includes(q) ||
      t.description.toLowerCase().includes(q) ||
      (t.category && t.category.toLowerCase().includes(q)),
    ).slice(0, 10);
  });

  readonly activeSearchId = computed(() =>
    this.activeSearchIndex() >= 0 ? this.searchSuggestionId(this.activeSearchIndex()) : null,
  );

  // === Filtered tabs (flat list, sin agrupar por categoría) ===
  readonly visibleTabs = computed(() => {
    const q = this.searchQuery().trim().toLowerCase();
    if (!q) return this.allTabs;
    return this.allTabs.filter(t => this.tabMatchesQuery(t, q));
  });

  categoryLabel(cat: string | undefined): string {
    return cat ? (CATEGORY_LABELS[cat] || cat) : 'Otros';
  }

  // === Search handlers ===
  onSearchChange(value: string) {
    this.searchQuery.set(value);
    this.activeSearchIndex.set(0);
    this.showSearchSuggestions.set(true);
  }

  clearSearch() {
    this.searchQuery.set('');
    this.activeSearchIndex.set(-1);
    this.showSearchSuggestions.set(false);
  }

  onSearchKeydown(event: KeyboardEvent) {
    // ⌘K / Ctrl+K para enfocar
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      this.searchInputRef?.nativeElement.focus();
      return;
    }
    if (event.key === 'Escape') {
      this.showSearchSuggestions.set(false);
      this.activeSearchIndex.set(-1);
    } else if (event.key === 'ArrowDown') {
      event.preventDefault();
      const max = this.filteredTabs().length - 1;
      this.activeSearchIndex.update(i => Math.min(i + 1, Math.max(max, 0)));
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      this.activeSearchIndex.update(i => Math.max(i - 1, 0));
    } else if (event.key === 'Enter') {
      event.preventDefault();
      const idx = this.activeSearchIndex();
      const tabs = this.filteredTabs();
      if (idx >= 0 && idx < tabs.length) {
        this.navigateTo(tabs[idx]);
      } else if (this.searchQuery().length > 0) {
        // Enter sin selección: si solo hay 1 match, navegar
        const matches = this.filteredTabs();
        if (matches.length === 1) this.navigateTo(matches[0]);
      }
    }
  }

  navigateTo(tab: TabDef) {
    this.showSearchSuggestions.set(false);
    this.searchQuery.set(tab.title);
    this.activeSearchIndex.set(-1);
    window.location.href = `/admin/maintainers/${tab.path}`;
  }

  searchSuggestionId(index: number): string {
    return `search-suggestion-${index}`;
  }

  private tabMatchesQuery(tab: TabDef, q: string): boolean {
    return tab.title.toLowerCase().includes(q) ||
      tab.path.toLowerCase().includes(q) ||
      tab.description.toLowerCase().includes(q) ||
      !!(tab.category && tab.category.toLowerCase().includes(q));
  }

  @HostListener('document:keydown', ['$event'])
  onDocumentKeydown(event: KeyboardEvent) {
    // ⌘K / Ctrl+K global para enfocar el search
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      this.searchInputRef?.nativeElement.focus();
    }
  }

  @HostListener('document:click', ['$event'])
  onDocumentClick(event: MouseEvent) {
    if (!this.host.nativeElement.contains(event.target as Node)) {
      this.showSearchSuggestions.set(false);
    }
  }
}
