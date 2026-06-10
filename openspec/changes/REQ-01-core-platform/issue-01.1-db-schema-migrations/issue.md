# issue-01.1-db-schema-migrations

**Origen:** `REQ-01-core-platform`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador de la plataforma
**Quiero** contar con un esquema de base de datos Postgres versionado mediante golang-migrate
**Para** tener una base sólida, repetible y auditable sobre la cual construir todos los módulos del sistema

## Criterios de aceptación

### Escenario 1: Migración inicial crea todas las tablas

```gherkin
Dado que la base de datos está vacía
Cuando ejecuto `migrate -path migrations -database "$DOMAIN_DATABASE_URL" up`
Entonces se crean las siguientes tablas:
  | organizations  | users          | api_keys      |
  | projects       | observations   | sessions      |
  | prompts        | knowledge_docs | skills        |
  | agents         | flows          | flow_runs     |
  | agent_runs     | crons          | webhooks      |
  | audit_log      | secrets        | cost_logs     |
  | skill_versions |                |               |
Y la extensión `pgvector` está habilitada
Y la extensión `pgcrypto` está habilitada
Y existe la tabla `project_templates`
Y existe la tabla `project_links`
Y existe la tabla `project_merges`
```

### Escenario 2: Cada tabla tiene columnas esperadas

```gherkin
Dado que las migraciones se ejecutaron correctamente
Cuando inspecciono la tabla `organizations`
Entonces existe la columna `id` de tipo `UUID` con default `gen_random_uuid()`
Y existe la columna `name` de tipo `VARCHAR(255)` con NOT NULL
Y existe la columna `slug` de tipo `VARCHAR(255)` con UNIQUE NOT NULL
Y existen las columnas `created_at` y `updated_at` de tipo `TIMESTAMPTZ` con default `NOW()`

Cuando inspecciono la tabla `users`
Entonces existe la columna `id` UUID PK
Y existe la columna `organization_id` UUID FK → organizations(id)
Y existe la columna `email` VARCHAR(255) UNIQUE NOT NULL
Y existe la columna `role` VARCHAR(50) NOT NULL default 'viewer'

Cuando inspecciono la tabla `api_keys`
Entonces existe la columna `id` UUID PK
Y existe la columna `key_hash` VARCHAR(255) NOT NULL
Y existe la columna `key_prefix` VARCHAR(10) NOT NULL
Y existe la columna `name` VARCHAR(255) NOT NULL
Y existe la columna `user_id` UUID FK → users(id)
Y existe la columna `organization_id` UUID FK → organizations(id)
Y existe la columna `expires_at` TIMESTAMPTZ nullable
Y existe la columna `revoked_at` TIMESTAMPTZ nullable

Cuando inspecciono la tabla `projects`
Entonces existe la columna `id` UUID PK
Y existe la columna `name` VARCHAR(255) NOT NULL
Y existe la columna `slug` VARCHAR(100) UNIQUE NOT NULL
Y existe la columna `repository_url` VARCHAR(500) nullable
Y existe la columna `template_id` UUID FK → project_templates(id) nullable
Y existe la columna `settings` JSONB NOT NULL DEFAULT '{}'
Y existe la columna `organization_id` UUID FK → organizations(id)

Cuando inspecciono la tabla `observations`
Entonces existe la columna `id` UUID PK
Y existe la columna `project_id` UUID FK → projects(id)
Y existe la columna `embedding` de tipo `vector(1536)` nullable
Y existe la columna `content` TEXT NOT NULL
Y existe la columna `content_tsv` TSVECTOR generado desde `content`

Cuando inspecciono la tabla `audit_log`
Entonces existe la columna `id` BIGSERIAL PK
Y existe la columna `actor_id` UUID NOT NULL
Y existe la columna `action` VARCHAR(50) NOT NULL
Y existe la columna `entity_type` VARCHAR(100) NOT NULL
Y existe la columna `entity_id` UUID NOT NULL
Y existe la columna `old_values` JSONB nullable
Y existe la columna `new_values` JSONB nullable
Y existe la columna `ip_address` VARCHAR(45) nullable
Y existe la columna `occurred_at` TIMESTAMPTZ NOT NULL default NOW()
```

### Escenario 3: Índices y constraints están presentes

```gherkin
Dado que las migraciones se ejecutaron
Cuando reviso los índices de `observations`
Entonces existe un índice GIN sobre `content_tsv`
Y existe un índice sobre `(project_id, created_at)`

Cuando reviso las constraints de `api_keys`
Entonces existe UNIQUE sobre `(organization_id, name)`

Cuando reviso las constraints de `users`
Entonces existe UNIQUE sobre `(organization_id, email)`

Cuando reviso las foreign keys de `flow_runs`
Entonces existe FK `flow_id` → flows(id) ON DELETE CASCADE
Y existe FK `triggered_by` → users(id) ON DELETE SET NULL
```

### Escenario 4: Migración down es limpia

```gherkin
Dado que las migraciones se ejecutaron up completamente
Cuando ejecuto `migrate -path migrations -database "$DOMAIN_DATABASE_URL" down -all`
Entonces la base de datos queda sin ninguna tabla del esquema de memoria
Y no hay errores de dependencias cíclicas
```

### Escenario 5: Migraciones son idempotentes

```gherkin
Dado que la migración 001 ya se ejecutó
Cuando intento ejecutarla nuevamente
Entonces golang-migrate la salta silenciosamente
Y no hay cambios en el esquema
```

## Análisis breve

- **Qué pide realmente:** Schema Postgres completo con migraciones versionadas, extensiones (pgvector, pgcrypto), índices, FKs, constraints, y TSVECTOR para búsqueda full-text.
- **Módulos sospechados:** `internal/database/migrations/`, `internal/models/`
- **Riesgos / dependencias:** Dependencia externa de `golang-migrate` y `pgvector`. El orden de creación de tablas debe respetar FKs. La migración down debe ser en orden inverso.
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
