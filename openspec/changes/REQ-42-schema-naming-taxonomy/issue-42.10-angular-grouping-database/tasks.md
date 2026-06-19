# Tasks: issue-42.10-angular-grouping-database

## Verificación previa (bloqueante)

- [ ] Confirmar que `dbschema.go:84` sigue excluyendo `schema_migrations` (`AND t.table_name <> 'schema_migrations'`)
- [ ] Confirmar que `GET /api/v1/admin/db-schema` devuelve `tbl.name` con el nombre real de tabla
- [ ] Confirmar que `seed_versions` aparece en la respuesta del endpoint (no filtrada en el SQL)
- [ ] Confirmar que `table_catalog` NO existe todavia (la rama de override debe tolerar `group_key` ausente)
- [ ] Verificar que los iconos cil-* del `PREFIX_GROUPS` estan registrados en `@coreui/icons-angular`
- [ ] Listar tablas reales que caen en "Otros" con prefijo puro (deuda hasta los renames de REQ-42)

## Frontend — `database-explorer.component.ts`

- [ ] Red: test que `groupedTables()` agrupa por prefijo (auth → "Autenticacion")
- [ ] Definir `interface GroupMeta { label; color; icon }` a nivel modulo
- [ ] Reemplazar `CATEGORY_META` (lineas 59-66) por el array `PREFIX_GROUPS` (22 grupos en orden de taxonomia)
- [ ] Definir `GROUP_INDEX = new Map(...)`, `OTHER` y `HIDDEN_TABLES = new Set(['schema_migrations'])`
- [ ] Implementar `groupKeyFor(tbl)`: (1) override `tbl.group_key`, (2) prefijo `name.split('_',1)[0]`, (3) `'__other__'`
- [ ] Reescribir `groupedTables()` (lineas 282-291): saltear `HIDDEN_TABLES`, agrupar por `groupKeyFor`, ordenar por `GROUP_INDEX` con "Otros" al final
- [ ] Confirmar que el template (`@for cat ... track cat.key`, linea 125) NO requiere cambios (sigue leyendo `cat.meta.icon/label`, `cat.tables`)
- [ ] Agregar entrada `seed` con label "Seeders corridos"
- [ ] Refactor: dejar `PREFIX_GROUPS`/`GROUP_INDEX` como constantes module-level inmutables

## Frontend — limpieza / deuda

- [ ] Marcar `tbl.category` (interface `TableInfo`, linea 49) como legacy en comentario (NO se borra en esta HU)
- [ ] NO tocar `dbschema.go` ni `categorize()` (se difiere a la HU backend de REQ-42)

## Tests

- [ ] Test unit: `auth_sessions` + `auth_events` → grupo `auth` / label "Autenticacion"
- [ ] Test unit: `schema_migrations` en la respuesta → NO aparece en ningun grupo
- [ ] Test unit: `seed_versions` → grupo `seed` / label "Seeders corridos"
- [ ] Test unit: `system_state` (prefijo no taxonomico) → grupo "Otros", renderizado al final
- [ ] Test unit: orden de grupos respeta el indice de `PREFIX_GROUPS` (users antes que auth antes que agent)
- [ ] Test unit: tabla con `group_key='auth'` override → cae en "Autenticacion" aunque su prefijo difiera
- [ ] Test unit: tabla SIN `group_key` (campo ausente) → usa prefijo (rama override inerte, no crashea)
- [ ] Test unit: con `query()='flow'` solo se renderiza el grupo "Flujos"

## Sabotaje (anti-falsos positivos)

Objetivo: probar que los tests detectan una regresion REAL, no que pasan por casualidad.

- [ ] **Sabotaje de ocultamiento:** en `groupedTables()`, COMENTAR la linea `if (HIDDEN_TABLES.has(tbl.name)) continue;` y correr la suite.
  - **Esperado:** el test "schema_migrations NO aparece en ningun grupo" debe FALLAR (schema_migrations apareceria en "Otros").
  - Si el test sigue verde tras quitar el guard → el test es un FALSO POSITIVO (no inyecta `schema_migrations` en el dataset de prueba). Corregir el fixture para que la respuesta incluya `schema_migrations`.
- [ ] **Sabotaje de override:** invertir la precedencia en `groupKeyFor` (calcular prefijo ANTES de chequear `override`) y correr.
  - **Esperado:** el test de override (`group_key='auth'` pisa prefijo) debe FALLAR.
- [ ] Restaurar ambos fixes → la suite vuelve a verde.

## Cierre

- [ ] `npm run lint` sin warnings (en `services/domain-admin/template`)
- [ ] `npm run test` verde (incluye los tests nuevos y los de sabotaje restaurados)
- [ ] `npm run build` OK
- [ ] Verificación manual: `/admin/database` muestra los grupos por funcionalidad, `schema_migrations` oculta, `seed_versions` bajo "Seeders corridos"
- [ ] NO hay migration en esta HU (frontend puro) — confirmar que no se creo ningun archivo en `internal/migrate/migrations/`
- [ ] Commit en rama `services` (Conventional Commits, español, SIN Co-Authored-By):
  `feat(req-42.10): agrupar tablas de /admin/database por funcionalidad (prefijo)`
- [ ] NO git push (repo local-only)
