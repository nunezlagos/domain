import { Component, inject, signal, computed, OnInit, ElementRef, HostListener } from '@angular/core';
import { CommonModule, DatePipe } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { HttpClient } from '@angular/common/http';

import {
  AlertComponent, CardComponent, CardBodyComponent, BadgeComponent,
  FormControlDirective, InputGroupComponent, InputGroupTextDirective,
} from '@coreui/angular';
import { IconDirective } from '@coreui/icons-angular';

import { apiBase } from '../../core/runtime-config';

interface RuntimeConfig {
  key: string;
  value: any;
  source: 'env' | 'flag' | 'database' | 'default';
  category: ConfigCategory;
  description?: string;
  updated_at?: string;
}

type ConfigCategory = 'http' | 'metrics' | 'database' | 's3' | 'security' | 'logging' | 'other';

const CATEGORY_META: Record<ConfigCategory, { label: string; icon: string; color: string }> = {
  http:     { label: 'HTTP',         icon: 'cilGlobeAlt',    color: 'info' },
  metrics:  { label: 'Metrics',      icon: 'cilGraph',       color: 'success' },
  database: { label: 'Database',     icon: 'cilStorage',     color: 'warning' },
  s3:       { label: 'S3 Storage',   icon: 'cilCloudUpload', color: 'primary' },
  security: { label: 'Security',     icon: 'cilFingerprint', color: 'danger' },
  logging:  { label: 'Logging',      icon: 'cilList',        color: 'secondary' },
  other:    { label: 'Other',        icon: 'cilSettings',    color: 'secondary' },
};

function inferCategory(key: string): ConfigCategory {
  const k = key.toLowerCase();
  if (k.includes('http_') || k.includes('http_port') || k.includes('http_bind')) return 'http';
  if (k.includes('metrics_')) return 'metrics';
  if (k.includes('database_') || k.includes('db_')) return 'database';
  if (k.includes('s3_')) return 's3';
  if (k.includes('password') || k.includes('admin') || k.includes('key') || k.includes('secret')) return 'security';
  if (k.includes('log')) return 'logging';
  return 'other';
}

// HU-41.4: Runtime Config — vista con typeahead input + filter chips.
// Read-only por seguridad; cambios requieren redeploy.
@Component({
  selector: 'app-runtime-config',
  standalone: true,
  imports: [
    CommonModule, DatePipe, FormsModule, IconDirective,
    AlertComponent, CardComponent, CardBodyComponent, BadgeComponent,
    FormControlDirective, InputGroupComponent, InputGroupTextDirective,
  ],
  template: `
    <c-alert color="warning" class="mb-3">
      <svg cIcon name="cilWarning" class="me-2"></svg>
      <strong>Solo lectura.</strong> Los cambios de config requieren redeploy del backend
      o de los feature flags vía MCP. Esta vista es para inspección.
    </c-alert>

    <!-- Typeahead input con suggestions -->
    <div class="position-relative mb-3">
      <c-input-group>
        <span cInputGroupText>
          <svg cIcon name="cilSearch"></svg>
        </span>
        <input
          #searchInput
          cFormControl
          type="search"
          role="combobox"
          aria-autocomplete="list"
          [attr.aria-expanded]="showSuggestions()"
          [attr.aria-activedescendant]="activeSuggestionId()"
          placeholder="Buscar config (ej: 'http', 'database', 's3')"
          [value]="query()"
          (input)="onQueryChange($any($event.target).value)"
          (focus)="onQueryFocus()"
          (keydown)="onKeydown($event)"
          autocomplete="off" />
        @if (query()) {
          <button
            cButton
            color="secondary"
            variant="outline"
            type="button"
            (click)="clearQuery()"
            aria-label="Limpiar búsqueda">
            <svg cIcon name="cilX"></svg>
          </button>
        }
      </c-input-group>

      <!-- Suggestions dropdown (typeahead) -->
      @if (showSuggestions() && suggestions().length > 0) {
        <ul
          class="config-suggestions list-unstyled shadow-sm"
          role="listbox">
          @for (cfg of suggestions(); track cfg.key; let i = $index) {
            <li
              [id]="suggestionId(i)"
              role="option"
              [class.active]="i === activeIndex()"
              [class.selected]="i === selectedIndex()"
              (click)="selectFromSuggestion(cfg)">
              <div class="d-flex justify-content-between align-items-center">
                <div>
                  <code class="fw-semibold">{{ cfg.key }}</code>
                  @if (cfg.description) {
                    <small class="text-body-secondary d-block">{{ cfg.description }}</small>
                  }
                </div>
                <c-badge [color]="badgeColorForCategory(cfg.category)">
                  {{ categoryLabel(cfg.category) }}
                </c-badge>
              </div>
            </li>
          }
        </ul>
      } @else if (showSuggestions() && query().length > 0) {
        <ul class="config-suggestions list-unstyled shadow-sm" role="listbox">
          <li class="empty text-body-secondary text-center py-3">
            Sin configs que matcheen "{{ query() }}"
          </li>
        </ul>
      }
    </div>

    <!-- Filter chips por categoría -->
    <div class="d-flex flex-wrap gap-2 mb-3 align-items-center">
      <small class="text-body-secondary text-uppercase me-1">Categoría:</small>
      <button
        cButton
        size="sm"
        [color]="activeCategories().size === 0 ? 'primary' : 'secondary'"
        [variant]="activeCategories().size === 0 ? 'solid' : 'outline'"
        (click)="clearCategories()">
        <svg cIcon name="cilGrid" class="me-1"></svg>
        Todos
        <span class="badge bg-light text-dark ms-1">{{ configs().length }}</span>
      </button>
      @for (cat of availableCategories(); track cat) {
        <button
          cButton
          size="sm"
          [color]="activeCategories().has(cat) ? CATEGORY_META[cat].color : 'secondary'"
          [variant]="activeCategories().has(cat) ? 'solid' : 'outline'"
          (click)="toggleCategory(cat)">
          <svg [cIcon]="CATEGORY_META[cat].icon" class="me-1"></svg>
          {{ CATEGORY_META[cat].label }}
          <span class="badge bg-light text-dark ms-1">{{ countByCategory(cat) }}</span>
        </button>
      }
      <span class="ms-auto text-body-secondary small">
        {{ filteredConfigs().length }} de {{ configs().length }} configs
      </span>
    </div>

    <!-- Cards grid -->
    <div class="row g-3">
      @for (cfg of filteredConfigs(); track cfg.key) {
        <div class="col-md-6">
          <c-card>
            <c-card-body>
              <div class="d-flex justify-content-between align-items-start gap-2">
                <code class="fs-6 text-break">{{ cfg.key }}</code>
                <c-badge [color]="badgeColorForCategory(cfg.category)">
                  {{ categoryLabel(cfg.category) }}
                </c-badge>
              </div>
              <pre class="bg-body-tertiary p-2 mt-2 mb-0 small rounded">{{ formatValue(cfg.value) }}</pre>
              @if (cfg.description) {
                <small class="text-body-secondary d-block mt-2">{{ cfg.description }}</small>
              }
            </c-card-body>
          </c-card>
        </div>
      } @empty {
        <div class="col-12">
          <c-alert color="info">
            <svg cIcon name="cilInfo" class="me-2"></svg>
            Sin configs que matcheen los filtros activos.
          </c-alert>
        </div>
      }
    </div>
  `,
  styles: [`
    pre { white-space: pre-wrap; word-break: break-all; }

    .config-suggestions {
      position: absolute;
      top: 100%;
      left: 0;
      right: 0;
      z-index: 1000;
      max-height: 320px;
      overflow-y: auto;
      background: var(--cui-body-bg);
      border: 1px solid var(--cui-border-color);
      border-radius: var(--cui-border-radius);
      margin-top: 4px;
      padding: 4px 0;
    }

    .config-suggestions li {
      padding: 8px 12px;
      cursor: pointer;
      border-left: 3px solid transparent;
    }

    .config-suggestions li:hover,
    .config-suggestions li.active {
      background: var(--cui-tertiary-bg);
      border-left-color: var(--cui-primary);
    }

    .config-suggestions li.selected {
      background: color-mix(in srgb, var(--cui-primary) 12%, transparent);
    }

    .config-suggestions li.empty {
      cursor: default;
    }

    .config-suggestions li.empty:hover {
      background: transparent;
      border-left-color: transparent;
    }
  `],
})
export class RuntimeConfigComponent implements OnInit {
  private readonly http = inject(HttpClient);
  private readonly host = inject(ElementRef<HTMLElement>);

  readonly CATEGORY_META = CATEGORY_META;

  readonly configs = signal<RuntimeConfig[]>([]);
  readonly query = signal('');
  readonly activeIndex = signal(-1);
  readonly selectedIndex = signal(-1);
  readonly showSuggestions = signal(false);
  readonly activeCategories = signal<Set<ConfigCategory>>(new Set());

  readonly availableCategories = computed<ConfigCategory[]>(() => {
    const cats = new Set<ConfigCategory>();
    for (const c of this.configs()) cats.add(c.category);
    return Array.from(cats).sort((a, b) => CATEGORY_META[a].label.localeCompare(CATEGORY_META[b].label));
  });

  readonly suggestions = computed<RuntimeConfig[]>(() => {
    const q = this.query().trim().toLowerCase();
    if (!q) return this.configs().slice(0, 8);
    return this.configs().filter(c =>
      c.key.toLowerCase().includes(q) ||
      c.description?.toLowerCase().includes(q),
    ).slice(0, 10);
  });

  readonly filteredConfigs = computed<RuntimeConfig[]>(() => {
    const q = this.query().trim().toLowerCase();
    const cats = this.activeCategories();
    return this.configs().filter(c => {
      // Filtro de texto
      const matchesText = !q ||
        c.key.toLowerCase().includes(q) ||
        c.description?.toLowerCase().includes(q);
      // Filtro de categoría
      const matchesCat = cats.size === 0 || cats.has(c.category);
      return matchesText && matchesCat;
    });
  });

  readonly activeSuggestionId = computed(() =>
    this.activeIndex() >= 0 ? this.suggestionId(this.activeIndex()) : null,
  );

  ngOnInit() { this.load(); }

  load() {
    // Por ahora exponemos configs hardcodeadas del sistema. En el futuro
    // el endpoint /api/v1/admin/runtime-configs/{key} retornará la config
    // persistida y la combinamos con env vars.
    const raw: Omit<RuntimeConfig, 'category'>[] = [
      { key: 'DOMAIN_HTTP_BIND',      value: '0.0.0.0',          source: 'env', description: 'Bind address del HTTP server' },
      { key: 'DOMAIN_HTTP_PORT',      value: 8000,               source: 'env', description: 'Puerto del HTTP server' },
      { key: 'http_timeout',          value: 30,                 source: 'env', description: 'Timeout en segundos para requests HTTP' },
      { key: 'DOMAIN_METRICS_ENABLED', value: true,               source: 'env', description: 'Habilita endpoint /metrics (Prometheus)' },
      { key: 'DOMAIN_METRICS_PORT',   value: 9090,               source: 'env', description: 'Puerto del servidor de métricas' },
      { key: 'DOMAIN_METRICS_BIND',   value: '0.0.0.0',          source: 'env', description: 'Bind address de /metrics' },
      { key: 'DOMAIN_DATABASE_URL',   value: 'postgres://app_user:****@postgres:5432/domain?sslmode=disable', source: 'env', description: 'Postgres URL (password redacted)' },
      { key: 'DOMAIN_DATABASE_AUTH_URL', value: 'postgres://app_admin:****@postgres:5432/domain?sslmode=disable', source: 'env', description: 'Auth pool URL (bypasea RLS para queries admin)' },
      { key: 'DOMAIN_S3_ENDPOINT',    value: 'http://minio:9000', source: 'env', description: 'Endpoint S3-compatible (MinIO)' },
      { key: 'DOMAIN_S3_BUCKET',      value: 'domain-attachments', source: 'env', description: 'Bucket para attachments/uploads' },
      { key: 'DOMAIN_S3_REGION',      value: 'us-east-1',        source: 'env', description: 'Región S3' },
      { key: 'APP_ADMIN_PASSWORD',    value: '****',             source: 'env', description: 'Password inicial del admin (cambiar tras primer login)' },
      { key: 'log_level',             value: 'info',             source: 'env', description: 'Nivel de log: debug | info | warn | error' },
    ];
    this.configs.set(raw.map(c => ({ ...c, category: inferCategory(c.key) })));
  }

  // === Typeahead handlers ===
  onQueryChange(value: string) {
    this.query.set(value);
    this.activeIndex.set(0);
    this.showSuggestions.set(true);
  }

  onQueryFocus() {
    if (this.query().length > 0 || this.configs().length > 0) {
      this.showSuggestions.set(true);
    }
  }

  clearQuery() {
    this.query.set('');
    this.activeIndex.set(-1);
    this.showSuggestions.set(false);
  }

  onKeydown(event: KeyboardEvent) {
    const sugs = this.suggestions();
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        this.activeIndex.update(i => Math.min(i + 1, sugs.length - 1));
        this.showSuggestions.set(true);
        break;
      case 'ArrowUp':
        event.preventDefault();
        this.activeIndex.update(i => Math.max(i - 1, 0));
        break;
      case 'Enter':
        event.preventDefault();
        const idx = this.activeIndex();
        if (idx >= 0 && idx < sugs.length) {
          this.selectFromSuggestion(sugs[idx]);
        }
        break;
      case 'Escape':
        this.showSuggestions.set(false);
        this.activeIndex.set(-1);
        break;
    }
  }

  selectFromSuggestion(cfg: RuntimeConfig) {
    this.selectedIndex.set(this.configs().findIndex(c => c.key === cfg.key));
    this.query.set(cfg.key);
    this.showSuggestions.set(false);
    this.activeIndex.set(-1);
    // Scroll al card seleccionado
    setTimeout(() => {
      const el = document.getElementById(`config-${this.cssId(cfg.key)}`);
      el?.scrollIntoView({ behavior: 'smooth', block: 'center' });
      el?.classList.add('highlight');
      setTimeout(() => el?.classList.remove('highlight'), 1500);
    }, 0);
  }

  @HostListener('document:click', ['$event'])
  onDocumentClick(event: MouseEvent) {
    if (!this.host.nativeElement.contains(event.target as Node)) {
      this.showSuggestions.set(false);
    }
  }

  // === Category filters ===
  toggleCategory(cat: ConfigCategory) {
    const next = new Set(this.activeCategories());
    if (next.has(cat)) next.delete(cat); else next.add(cat);
    this.activeCategories.set(next);
  }

  clearCategories() {
    this.activeCategories.set(new Set());
  }

  countByCategory(cat: ConfigCategory): number {
    return this.configs().filter(c => c.category === cat).length;
  }

  // === Helpers ===
  suggestionId(index: number): string {
    return `config-suggestion-${index}`;
  }

  cssId(key: string): string {
    return key.replace(/[^a-z0-9]/gi, '-');
  }

  categoryLabel(cat: ConfigCategory): string {
    return CATEGORY_META[cat].label;
  }

  badgeColorForCategory(cat: ConfigCategory): string {
    return CATEGORY_META[cat].color;
  }

  formatValue(v: any): string {
    if (typeof v === 'string') return v;
    return JSON.stringify(v, null, 2);
  }
}
