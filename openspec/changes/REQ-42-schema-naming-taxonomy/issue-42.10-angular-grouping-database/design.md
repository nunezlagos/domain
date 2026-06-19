# Design: issue-42.10-angular-grouping-database

## Decisión arquitectónica

**El agrupamiento se resuelve 100% en el FRONTEND por PREFIJO del nombre de tabla.** La respuesta de `GET /api/v1/admin/db-schema` ya trae `tbl.name`; el grupo se deriva del primer token antes del `_`. Esto evita tocar el backend, evita una migration y deja el campo `category` (legacy, 6 buckets) sin uso pero sin romperlo.

Razones:
- **Menor riesgo:** sin deploy de backend, sin migration. Un cambio en un solo archivo Angular.
- **Fuente de verdad única:** el prefijo del nombre ES la taxonomia de REQ-42 (cada tabla lleva prefijo de su funcionalidad). Derivar del nombre evita un mapa hardcodeado que se desincronice.
- **Override opcional sin acoplar:** si en el futuro la HU 42.1 crea `table_catalog` y el backend agrega `group_key` a `TableInfo`, `groupKeyFor` lo usa ANTES del prefijo. Mientras `group_key` no venga, esa rama queda inerte.

**Decisión de NO tocar `dbschema.go` en esta HU.** El SELECT (linea 84) ya excluye `schema_migrations`. `categorize()` y `tbl.category` quedan como código legacy desacoplado. NO se borran aqui para acotar el blast radius; se documenta como deuda (ver Riesgos). La limpieza de `categorize()` se difiere a la HU del backend de REQ-42 que toque `dbschema.go`.

## Snippet concreto (database-explorer.component.ts)

Reemplaza `CATEGORY_META` (lineas 59-66) y `groupedTables()` (lineas 282-291).

```typescript
// ---- Agrupamiento por PREFIJO (taxonomia REQ-42) ----
// El orden del array = orden de render. schema_migrations -> HIDDEN.
interface GroupMeta { label: string; color: string; icon: string; }

const PREFIX_GROUPS: { prefix: string; meta: GroupMeta }[] = [
  { prefix: 'users',        meta: { label: 'Usuarios y RBAC',         color: 'primary',   icon: 'cilUser' } },
  { prefix: 'auth',         meta: { label: 'Autenticacion',           color: 'primary',   icon: 'cilLockLocked' } },
  { prefix: 'agent',        meta: { label: 'Agentes',                 color: 'info',      icon: 'cilRobot' } },
  { prefix: 'flow',         meta: { label: 'Flujos',                  color: 'info',      icon: 'cilFork' } },
  { prefix: 'skill',        meta: { label: 'Skills',                  color: 'info',      icon: 'cilLightbulb' } },
  { prefix: 'mcp',          meta: { label: 'Servidores MCP',          color: 'info',      icon: 'cilLan' } },
  { prefix: 'prompt',       meta: { label: 'Prompts',                 color: 'info',      icon: 'cilSpeech' } },
  { prefix: 'project',      meta: { label: 'Proyectos y tickets',     color: 'success',   icon: 'cilFolder' } },
  { prefix: 'sdd',          meta: { label: 'SDD (especificacion)',    color: 'secondary', icon: 'cilFile' } },
  { prefix: 'tdd',          meta: { label: 'TDD y verificacion',      color: 'secondary', icon: 'cilCheckCircle' } },
  { prefix: 'issue',        meta: { label: 'Issues / Historias',      color: 'secondary', icon: 'cilTask' } },
  { prefix: 'knowledge',    meta: { label: 'Base de conocimiento',    color: 'warning',   icon: 'cilLibrary' } },
  { prefix: 'webhook',      meta: { label: 'Webhooks',                color: 'warning',   icon: 'cilBolt' } },
  { prefix: 'external',     meta: { label: 'Integraciones externas',  color: 'warning',   icon: 'cilCloud' } },
  { prefix: 'cron',         meta: { label: 'Tareas programadas',      color: 'warning',   icon: 'cilClock' } },
  { prefix: 'usage',        meta: { label: 'Uso y cuotas',            color: 'warning',   icon: 'cilChartLine' } },
  { prefix: 'notification', meta: { label: 'Notificaciones',          color: 'warning',   icon: 'cilBell' } },
  { prefix: 'runner',       meta: { label: 'Runners self-hosted',     color: 'dark',      icon: 'cilTerminal' } },
  { prefix: 'platform',     meta: { label: 'Politicas de plataforma', color: 'dark',      icon: 'cilShieldAlt' } },
  { prefix: 'file',         meta: { label: 'Archivos adjuntos',       color: 'dark',      icon: 'cilPaperclip' } },
  { prefix: 'audit',        meta: { label: 'Auditoria y actividad',   color: 'dark',      icon: 'cilList' } },
  { prefix: 'seed',         meta: { label: 'Seeders corridos',        color: 'success',   icon: 'cilStorage' } },
];
const GROUP_INDEX = new Map(PREFIX_GROUPS.map((g, i) => [g.prefix, i]));
const OTHER: GroupMeta = { label: 'Otros', color: 'dark', icon: 'cilSettings' };

// schema_migrations no deberia llegar (lo filtra el SQL en dbschema.go:84),
// pero lo ocultamos defensivamente por si alguien quita ese WHERE.
const HIDDEN_TABLES = new Set<string>(['schema_migrations']);

// grupo por: (1) override table_catalog si viene (42.1), (2) prefijo, (3) '__other__'
function groupKeyFor(tbl: TableInfo): string {
  const override = (tbl as { group_key?: string }).group_key;
  if (override) return override;                       // override DB opcional (42.1)
  const prefix = tbl.name.split('_', 1)[0];
  return GROUP_INDEX.has(prefix) ? prefix : '__other__';
}
```

```typescript
// computed que reemplaza el groupedTables() actual
readonly groupedTables = computed(() => {
  const buckets = new Map<string, { key: string; meta: GroupMeta; tables: TableInfo[] }>();
  for (const tbl of this.filteredTables()) {
    if (HIDDEN_TABLES.has(tbl.name)) continue;          // oculta schema_migrations
    const key = groupKeyFor(tbl);
    const meta = key === '__other__'
      ? OTHER
      : PREFIX_GROUPS[GROUP_INDEX.get(key)!].meta;
    if (!buckets.has(key)) buckets.set(key, { key, meta, tables: [] });
    buckets.get(key)!.tables.push(tbl);
  }
  // ordenar por la taxonomia; 'Otros' (no indexado) al final
  return [...buckets.values()].sort((a, b) => {
    const ai = GROUP_INDEX.has(a.key) ? GROUP_INDEX.get(a.key)! : 999;
    const bi = GROUP_INDEX.has(b.key) ? GROUP_INDEX.get(b.key)! : 999;
    return ai - bi;
  });
});
```

El template (`@for (cat of groupedTables(); track cat.key)` en linea 125) NO cambia: sigue leyendo `cat.meta.icon`, `cat.meta.label`, `cat.tables`. El `track cat.key` ahora usa el prefijo (string estable).

## DDL / migration

**Ninguna.** Esta HU no crea ni altera tablas. La tabla opcional `table_catalog` que habilitaria el override por `group_key` pertenece a la HU 42.1 (no a esta). Sin 42.1, la rama `if (override)` de `groupKeyFor` nunca se activa.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|---|---|
| El SQL de `loadDBSchema` (dbschema.go:84) ya excluye `schema_migrations`, asi que `HIDDEN_TABLES` es redundante | Se mantiene como defensa: si alguien quita el `WHERE`, el front la sigue tapando. Costo cero. |
| `categorize()` del backend y `tbl.category` quedan como código muerto tras mover el agrupamiento al front | Se documenta como deuda; NO se borra en esta HU (acotar blast radius). La limpieza se difiere a la HU backend de REQ-42. |
| Tablas sin prefijo de la taxonomia (`users`, `issues`, `agents`, `flows`, `skills`, `prompts`, `projects`, `crons`, `webhooks`, `clients`, `observations`, `enrollment_tokens`) caen en "Otros" con prefijo puro | Esperado y transitorio: se resuelve cuando REQ-42 aplica los renames (000147+) o via override `table_catalog` (42.1). El grupo "Otros" al final absorbe lo no clasificado sin romper la UI. |
| `name.split('_', 1)` toma el primer token; nombres marcados para DROP como `cost_logs` o `system_state` caerian en grupos inexistentes (`cost`, `system`) → "Otros" | Tras los drops de REQ-42 desaparecen. Antes, "Otros" los contiene; no hay crash. |
| `group_key` (override) podria venir de un `table_catalog` mal seedeado y "esconder" una tabla en un grupo inesperado | El override gana siempre por diseño. La validacion del seed es responsabilidad de 42.1, no de esta HU. Aqui solo se consume el campo si esta presente. |
| Iconos cil-* del `PREFIX_GROUPS` podrian no estar registrados en el set de `@coreui/icons-angular` | Verificacion previa (checklist). Si falta alguno, el `svg cIcon` no rompe el render (queda vacio); igual conviene confirmar antes. |
| `track cat.key` cambia de category-key a prefijo: si dos grupos colisionaran de key, Angular re-renderiza | Imposible por construccion: las keys de `PREFIX_GROUPS` son unicas y `__other__` es un literal reservado. |

## TDD plan (frontend, Vitest)

1. **Red:** test que `groupedTables()` agrupa `auth_sessions` y `auth_events` bajo el grupo `auth` con label "Autenticacion".
2. **Green:** implementar `PREFIX_GROUPS` + `groupKeyFor` + nuevo `groupedTables`.
3. **Red:** test que `schema_migrations` NO aparece en ningun grupo (aunque venga en la respuesta).
4. **Green:** `HIDDEN_TABLES.has(tbl.name)` → `continue`.
5. **Red:** test que `seed_versions` cae en el grupo `seed` con label "Seeders corridos".
6. **Green:** entrada `seed` en `PREFIX_GROUPS`.
7. **Red:** test que una tabla con prefijo no-taxonomico (`system_state`) cae en "Otros" y "Otros" queda al final.
8. **Green:** rama `__other__` + sort con `999`.
9. **Red:** test que un `tbl.group_key='auth'` override pisa el prefijo (ej. una tabla `weird_thing` con group_key auth aparece en "Autenticacion").
10. **Green:** rama `if (override) return override`.
11. **Refactor:** extraer `PREFIX_GROUPS`/`GROUP_INDEX` a constantes module-level (ya lo estan).
12. **Sabotaje:** ver tasks.md — quitar el `continue` de `HIDDEN_TABLES` y confirmar que el test de ocultamiento FALLA.
