# issue-42.10-angular-grouping-database

**Origen:** `REQ-42-schema-naming-taxonomy`
**Prioridad tentativa:** media
**Tipo:** feature (frontend / tooling)

## Historia de usuario

**Como** administrador que entra a `/admin/database`
**Quiero** ver las tablas del schema agrupadas por funcionalidad (Auth, Agentes, Flujos, Skills, ...) derivada del prefijo real del nombre, con `schema_migrations` OCULTA y `seed_versions` VISIBLE bajo "Seeders corridos"
**Para** navegar el dominio por áreas reales (la taxonomía de ~22 grupos de REQ-42) en lugar de los 6 buckets gruesos hardcodeados actuales, y entender de un vistazo a qué subsistema pertenece cada tabla

## Criterios de aceptación

```gherkin
Feature: Agrupacion de tablas por funcionalidad en /admin/database

  Background:
    Given el admin esta autenticado con rol con acceso a /admin/database
    And el endpoint GET /api/v1/admin/db-schema devuelve la lista de tablas

  Scenario: Las tablas se ven agrupadas por funcionalidad
    When navego a /admin/database
    Then veo encabezados de grupo derivados del prefijo: "Autenticacion", "Agentes", "Flujos", "Skills", "Servidores MCP", "Prompts", "Proyectos y tickets", "SDD (especificacion)", "TDD y verificacion", "Issues / Historias", "Base de conocimiento", "Webhooks", "Integraciones externas", "Tareas programadas", "Uso y cuotas", "Notificaciones", "Runners self-hosted", "Politicas de plataforma", "Archivos adjuntos", "Auditoria y actividad", "Seeders corridos"
    And cada tabla aparece dentro del grupo que corresponde al primer token de su nombre antes del "_"
    And los grupos se renderizan en el orden de la taxonomia (Usuarios/Auth primero, "Otros" al final)

  Scenario: schema_migrations queda oculta
    When navego a /admin/database
    Then NO veo la tabla schema_migrations en ningun grupo
    And no se renderiza ningun grupo "Interno"

  Scenario: seed_versions se muestra bajo "Seeders corridos"
    Given la tabla seed_versions existe en el schema
    When navego a /admin/database
    Then veo un grupo "Seeders corridos"
    And la tabla seed_versions aparece dentro de ese grupo

  Scenario: Tabla sin prefijo de la taxonomia cae en "Otros"
    Given existe una tabla cuyo primer token NO esta en la taxonomia (ej. system_state)
    When navego a /admin/database
    Then esa tabla aparece en el grupo "Otros"
    And el grupo "Otros" se renderiza al final

  Scenario: Override opcional via table_catalog (depende de 42.1)
    Given el backend incluye group_key en la respuesta de db-schema
    When una tabla trae group_key="auth"
    Then esa tabla se agrupa en "Autenticacion" aunque su prefijo no sea auth_
    And el override de group_key gana sobre la derivacion por prefijo

  Scenario: El buscador respeta el agrupamiento
    When escribo "flow" en el buscador
    Then solo veo el grupo "Flujos" con sus tablas filtradas
    And los grupos sin coincidencias no se renderizan
```

## Componentes a tocar

| Archivo | Cambio |
|---|---|
| `services/domain-admin/template/src/app/views/admin-database/database-explorer.component.ts` | Reemplazar `CATEGORY_META` (6 buckets) y `groupedTables()` (que itera `tbl.category`) por un mapa `PREFIX_GROUPS` (prefijo → {label,color,icon}) + `groupKeyFor(tbl)` que parte el nombre por el primer `_`. Agregar `HIDDEN_TABLES` (defensivo para `schema_migrations`) y entrada `seed` con label "Seeders corridos". Soportar override opcional `tbl.group_key`. |
| `services/domain-backend/internal/api/handler/dbschema.go` | NO se toca en esta HU (opcional). El SELECT ya excluye `schema_migrations` (linea 84). El campo `category` queda como hint legacy sin uso. Si 42.1 crea `table_catalog`, esa HU agrega `group_key`/`label` a `TableInfo`. |

**Sin migration.** Esta HU es 100% frontend. El override por `table_catalog` (campo `group_key` en la respuesta) DEPENDE de la HU 42.1; mientras 42.1 no exista, `groupKeyFor` cae siempre a la derivacion por prefijo y la rama de override queda inerte (defensiva).

## Análisis breve

- **Qué pide realmente:** Mover el agrupamiento del `/admin/database` de 6 categorias gruesas hardcodeadas a la taxonomia funcional de REQ-42 (~22 grupos), derivando el grupo por PREFIJO del nombre de tabla (primer token antes de `_`). Ocultar `schema_migrations`, mostrar `seed_versions` como "Seeders corridos".
- **Por qué frontend y no backend:** la respuesta de `db-schema` ya trae `tbl.name`; el prefijo se calcula en el cliente sin tocar el endpoint. Mover la decision al frontend es lo de menor riesgo (no deploy de backend, no migration). El campo `category` del backend queda como hint legacy.
- **Verificado contra el código real:**
  - `dbschema.go:84` → el SELECT ya excluye `schema_migrations` con `AND t.table_name <> 'schema_migrations'`. Ocultarla en el front es defensivo/redundante.
  - `database-explorer.component.ts:59-66` → `CATEGORY_META` con 6 keys (core/resources/observability/system/sdd/other).
  - `database-explorer.component.ts:282-291` → `groupedTables()` itera `CATEGORY_META` y agrupa por `tbl.category`.
  - NO existe tabla `table_catalog` en el schema real (verificado en `schema_introspect.txt`).
  - `seed_versions` HOY cae en el bucket `other` (visible pero sin label propio).
- **Módulos a tocar:** solo `database-explorer.component.ts`. El `routes.ts` de `admin-database` no cambia.
- **Riesgos / dependencias:** tablas sin prefijo canonico de la taxonomia (`users`, `issues`, `agents`, `flows`, `skills`, `prompts`, `projects`, `crons`, `webhooks`, `clients`, `observations`, `enrollment_tokens`) caen en "Otros" con la logica de prefijo puro. Se resuelven cuando REQ-42 aplica los renames (000147+) o via override `table_catalog` (42.1). Antes de los renames, la UI mostrara algunos grupos transitorios y "Otros" poblado.
- **Esfuerzo tentativo:** S

## Verificación previa

- [ ] Confirmar que `dbschema.go` sigue excluyendo `schema_migrations` en el SELECT (linea 84) y que el front no debe asumir lo contrario
- [ ] Confirmar que la respuesta de `GET /api/v1/admin/db-schema` trae `tbl.name` con el nombre real de la tabla (no truncado, no aliasado)
- [ ] Confirmar que `table_catalog` NO existe todavia (la rama de override debe ser tolerante a `group_key` ausente)
- [ ] Confirmar que `seed_versions` aparece en la respuesta del endpoint (no esta filtrada en el SQL)
- [ ] Listar todas las tablas reales y mapear cuales caen en "Otros" con prefijo puro (deuda hasta los renames de REQ-42)
- [ ] Confirmar que `IconDirective` tiene disponibles los iconos cil-* usados en `PREFIX_GROUPS` (cilRobot, cilFork, cilLightbulb, cilLan, etc.)

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
