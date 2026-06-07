# HU-01.8-platform-policies

**Origen:** `REQ-01-core-platform`
**Persona:** platform-engineer, security-officer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** plataforma autónoma
**Quiero** que todas las policies (conventions DB, API, security, testing, observability, migrations, clean-architecture, etc.) vivan en BD como source of truth
**Para** que agentes/CLI/MCP las consulten en runtime y se garantice consistencia, sin depender de archivos sueltos

## Modelo

- Tabla `platform_policies` con cada policy como row
- Body markdown + structured JSONB (parsed) + metadata
- Versionado: cada edit crea nueva version (igual que prompts)
- CLI `domain policies export-md` regenera `.claude/rules/*.md` desde BD (DB es SOT)
- CLI `domain policies import-md` carga `.claude/rules/*.md` a DB (usado por seeders inicial + por dev)
- MCP tool `domain_policy_get(slug)` accesible por agentes

## Criterios de aceptación

### Escenario 1: Schema y CRUD

```gherkin
Dado que existe tabla `platform_policies` con (id, slug, name, kind, body_md, body_structured JSONB, version, is_active, created_at, ...)
Cuando POST /api/v1/platform/policies con `{slug:"db", body_md:"...", kind:"convention"}`
Entonces se crea row + audit log
Y la response incluye id + versión
```

### Escenario 2: Kinds soportados

```gherkin
Dado que existen kinds:
  | kind            | descripción                                            |
  | convention      | Naming, types, patterns (db, api, testing, etc.)       |
  | security_rule   | Reglas seguridad (security.md)                         |
  | architecture    | Clean architecture                                     |
  | sdd_workflow    | SDD process                                            |
  | observability   | Logging/metrics/tracing rules                          |
  | migration_rule  | Migration conventions                                  |
  | linter_config   | Configs de linters (squawk, custom)                    |
Cuando consulto GET /api/v1/platform/policies?kind=convention
Entonces devuelve solo policies de ese kind
```

### Escenario 3: Versionado

```gherkin
Dado que existe policy `db` v3 con body X
Cuando PATCH la policy con body Y
Entonces v3 queda como histórico
Y v4 active con body Y
Y rows previas tienen `is_active = false`
```

### Escenario 4: Export a markdown

```gherkin
Dado que existen N policies en DB
Cuando ejecuto `domain policies export-md --to ./.claude/rules/`
Entonces se genera 1 archivo .md por policy active
Y filename = `<slug>.md`
Y contenido = body_md actual + header con `<!-- domain-policy: slug=X version=N generated_at=T -->`
Y el comando es idempotente
```

### Escenario 5: Import desde markdown

```gherkin
Dado que existen `.claude/rules/*.md`
Cuando ejecuto `domain policies import-md --from ./.claude/rules/`
Entonces se hace UPSERT en `platform_policies`:
  - si slug no existe → create new
  - si slug existe → bump version si body cambió
Y reporte de diff por policy
```

### Escenario 6: MCP tool domain_policy_get

```gherkin
Dado que un agente Claude/Cline conectado al MCP
Cuando invoca tool `domain_policy_get(slug="db")`
Entonces devuelve `{slug, kind, version, body_md, body_structured}`
Y respeta timeout 2s + circuit breaker (HU-12.6)
Y cachea en memoria del proceso MCP 5min
```

### Escenario 7: MCP tool domain_policies_search

```gherkin
Dado que agente quiere encontrar policy relevante
Cuando invoca `domain_policies_search(query="cómo nombrar tabla")`
Entonces FTS sobre body_md devuelve top-3 con score
Y permite navegar al detalle
```

### Escenario 8: Audit y rollback

```gherkin
Dado que admin edita policy y rompe algo
Cuando GET /api/v1/platform/policies/db/versions
Entonces devuelve todas las versiones históricas
Y POST /policies/db/versions/3/activate revierte a v3
Y audit log de rollback
```

### Escenario 9: Source of truth flow

```gherkin
Dado que es deploy productivo
Cuando el binary boot:
  1. golang-migrate up
  2. seeders run (HU-01.7) → import .md embebidos a DB con UPSERT
  3. La DB queda con catalog inicial
Y después en runtime:
  - admins editan via API → DB cambia
  - `domain policies export-md` periódico → regenera .claude/rules/*.md para developers locales
  - cambios en DB son source of truth
```

### Escenario 10: Renderer estructurado

```gherkin
Dado que policy markdown tiene secciones structured (e.g. tabla naming)
Cuando se procesa import-md
Entonces parser extrae estructuras y popula `body_structured` JSONB:
  ```json
  {
    "naming_rules": [{"pattern": "^[a-z][a-z0-9_]*$", "scope": "tables"}],
    "type_mappings": [{"prefer": "JSONB", "deprecate": "JSON"}],
    "..."
  }
```
Y MCP tool `domain_policy_get_structured(slug)` devuelve solo `body_structured`
Y linters HU-25.13/HU-13.9 leen desde body_structured (no parsean markdown raw)
```

## Análisis breve

- **Qué pide:** tabla + CRUD + version + export/import md + 2 MCP tools + audit/rollback + structured parser
- **Esfuerzo:** L
- **Riesgos:** drift DB vs filesystem si dev edita md local sin import; structured parser frágil para formato markdown libre
