import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { NoopAnimationsModule } from '@angular/platform-browser/animations';

import { IconSetService } from '@coreui/icons-angular';
import { iconSubset } from '../../icons/icon-subset';
import { DatabaseExplorerComponent } from './database-explorer.component';

// Fixture minimo de TableInfo: solo los campos que toca el agrupamiento.
function tbl(name: string, extra: Record<string, unknown> = {}) {
  return {
    name,
    schema: 'public',
    row_count: 0,
    size_bytes: 0,
    has_created_at: true,
    has_updated_at: true,
    has_status: false,
    columns: [],
    indexes: [],
    foreign_keys: [],
    category: 'other',
    ...extra,
  } as never;
}

describe('DatabaseExplorerComponent — agrupamiento REQ-42.10', () => {
  let component: DatabaseExplorerComponent;
  let fixture: ComponentFixture<DatabaseExplorerComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [NoopAnimationsModule, DatabaseExplorerComponent],
      providers: [
        IconSetService,
        provideHttpClient(),
        provideHttpClientTesting(),
      ],
    }).compileComponents();

    const iconSetService = TestBed.inject(IconSetService);
    iconSetService.icons = { ...iconSubset };

    fixture = TestBed.createComponent(DatabaseExplorerComponent);
    component = fixture.componentInstance;
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });

  it('agrupa auth_sessions y auth_events bajo "auth" / "Autenticacion"', () => {
    component.tables.set([tbl('auth_sessions'), tbl('auth_events')]);
    const groups = component.groupedTables();
    const auth = groups.find(g => g.key === 'auth');
    expect(auth).toBeTruthy();
    expect(auth!.meta.label).toBe('Autenticacion');
    expect(auth!.tables.map(t => t.name).sort()).toEqual(['auth_events', 'auth_sessions']);
  });

  it('schema_migrations NO aparece en ningun grupo aunque venga en la respuesta', () => {
    component.tables.set([tbl('schema_migrations'), tbl('auth_sessions')]);
    const groups = component.groupedTables();
    const all = groups.flatMap(g => g.tables.map(t => t.name));
    expect(all).not.toContain('schema_migrations');
    // no se materializa un grupo "Interno"/__other__ solo por schema_migrations
    expect(groups.some(g => g.tables.some(t => t.name === 'schema_migrations'))).toBe(false);
  });

  it('seed_versions cae en el grupo "seed" / "Seeders corridos"', () => {
    component.tables.set([tbl('seed_versions')]);
    const groups = component.groupedTables();
    const seed = groups.find(g => g.key === 'seed');
    expect(seed).toBeTruthy();
    expect(seed!.meta.label).toBe('Seeders corridos');
    expect(seed!.tables[0].name).toBe('seed_versions');
  });

  it('una tabla con prefijo no-taxonomico (system_state) cae en "Otros" al final', () => {
    component.tables.set([tbl('system_state'), tbl('auth_sessions')]);
    const groups = component.groupedTables();
    const last = groups[groups.length - 1];
    expect(last.meta.label).toBe('Otros');
    expect(last.tables.map(t => t.name)).toContain('system_state');
  });

  it('respeta el orden de la taxonomia: users antes que auth antes que agent', () => {
    component.tables.set([tbl('agent_runs'), tbl('auth_sessions'), tbl('users_roles')]);
    const keys = component.groupedTables().map(g => g.key);
    expect(keys.indexOf('users')).toBeLessThan(keys.indexOf('auth'));
    expect(keys.indexOf('auth')).toBeLessThan(keys.indexOf('agent'));
  });

  it('override group_key="auth" pisa el prefijo (cae en "Autenticacion")', () => {
    component.tables.set([tbl('weird_thing', { group_key: 'auth' })]);
    const groups = component.groupedTables();
    const auth = groups.find(g => g.key === 'auth');
    expect(auth).toBeTruthy();
    expect(auth!.meta.label).toBe('Autenticacion');
    expect(auth!.tables[0].name).toBe('weird_thing');
  });

  it('tabla SIN group_key (campo ausente) usa el prefijo y no crashea', () => {
    component.tables.set([tbl('flow_runs')]);
    const groups = component.groupedTables();
    expect(groups.find(g => g.key === 'flow')).toBeTruthy();
  });

  it('con query()="flow" solo se renderiza el grupo "Flujos"', () => {
    component.tables.set([tbl('flow_runs'), tbl('auth_sessions'), tbl('agent_runs')]);
    component.query.set('flow');
    const groups = component.groupedTables();
    expect(groups.length).toBe(1);
    expect(groups[0].key).toBe('flow');
    expect(groups[0].meta.label).toBe('Flujos');
  });
});
