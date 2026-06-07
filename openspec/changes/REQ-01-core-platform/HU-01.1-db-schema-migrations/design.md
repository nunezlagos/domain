# Design: HU-01.1-db-schema-migrations

## Decisión arquitectónica

**Driver de migraciones:** golang-migrate/migrate v4 con driver postgres.
**Formato:** SQL plano, versionado, up/down.
**Ubicación:** `migrations/` en la raíz del proyecto.

Razones:
- golang-migrate es el estándar de facto en Go para migraciones
- SQL plano permite DDL completo sin abstracciones
- Idempotente por diseño (trackea migraciones aplicadas en tabla `schema_migrations`)
- Soporta driver postgres nativo y transacciones implícitas

## Alternativas descartadas

- **GORM AutoMigrate:** No da control fino sobre índices, FKs, extensiones. No versiona down.
- **sqlc + manual:** Demasiado boilerplate, sin versionado built-in.
- **Pressly/goose:** Similar a golang-migrate pero menos adopción y ecosistema.
- **Ent (Facebook):** ORM con migrations, pero agrega dependencia pesada y curva de aprendizaje.

## Diagrama

```
┌──────────────────────────────────────────────────┐
│                  organizations                    │
├──────────────────────────────────────────────────┤
│ PK id UUID                                       │
│ name VARCHAR(255) NOT NULL                       │
│ slug VARCHAR(255) UNIQUE NOT NULL               │
│ created_at TIMESTAMPTZ DEFAULT NOW()             │
│ updated_at TIMESTAMPTZ DEFAULT NOW()             │
└────────┬─────────────────────────────────────────┘
         │ 1
         │
    ┌────┴─────────────────────────────────────────┐
    │                  users                        │
    ├───────────────────────────────────────────────┤
    │ PK id UUID                                    │
    │ FK organization_id → organizations(id)        │
    │ email VARCHAR(255) NOT NULL                   │
    │ UNIQUE(organization_id, email)                │
    │ role VARCHAR(50) DEFAULT 'viewer'             │
    │ created_at, updated_at                        │
    └────┬──────────────────────────────────────────┘
         │ 1
         │
    ┌────┴──────────────────────────────────────────┐
    │               api_keys                        │
    ├───────────────────────────────────────────────┤
    │ PK id UUID                                    │
    │ FK organization_id → organizations(id)        │
    │ FK user_id → users(id)                        │
    │ key_hash VARCHAR(255) NOT NULL                │
    │ key_prefix VARCHAR(10) NOT NULL               │
    │ name VARCHAR(255) NOT NULL                    │
    │ UNIQUE(organization_id, name)                 │
    │ expires_at TIMESTAMPTZ NULL                   │
    │ revoked_at TIMESTAMPTZ NULL                   │
    │ created_at, updated_at                        │
    └───────────────────────────────────────────────┘

      organizations 1 ──→ N projects
      projects: repository_url, settings (JSONB), template_id → project_templates(id)
      project_templates: name, description, is_default, settings (JSONB), default_skills
      project_links: (project_id, linked_project_id, access_level)
      project_merges: (source_project_id, target_project_id, merge_log)
      projects     1 ──→ N observations
      observations: embedding VECTOR(1536), content_tsv TSVECTOR (GIN)
      projects     1 ──→ N prompts
      projects     1 ──→ N knowledge_docs
      projects     1 ──→ N project_merges (source)
      projects     1 ──→ N project_merges (target)
      projects     N ──→ N linked_projects (via project_links)
     organizations 1 ──→ N skills
     skills       1 ──→ N skill_versions
     organizations 1 ──→ N agents
     organizations 1 ──→ N flows
     flows        1 ──→ N flow_runs
     flow_runs    1 ──→ N agent_runs
     agents       1 ──→ N agent_runs
     organizations 1 ──→ N crons
     organizations 1 ──→ N webhooks
     audit_log: append-only, no FKs (performance)
     organizations 1 ──→ N secrets
     organizations 1 ──→ N cost_logs
```

## TDD plan

1. **Test migración up:** Conectar a Postgres de test, ejecutar `migrate up`, verificar con queries `information_schema.tables` que existan las 23 tablas.
2. **Test migración down:** Ejecutar `migrate down -all`, verificar que `information_schema.tables` no devuelva ninguna.
3. **Test idempotencia:** Ejecutar `migrate up` dos veces seguidas, verificar que `schema_migrations` count no cambia en la segunda.
4. **Test constraints:** Insertar registros violating FK, UNIQUE, NOT NULL — esperar error.
5. **Test pgvector:** Insertar vector, hacer SELECT por distancia coseno.
6. **Test TSVECTOR:** Insertar texto, verificar que content_tsv se genera automáticamente, hacer búsqueda full-text.

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| pgvector no instalado | Media | Alto | Documentar prerequisito; fallar temprano con error claro |
| Migración out of order en CI/CD | Baja | Alto | Usar `migrate -lock-timeout` y migraciones atómicas |
| Datos existentes al agregar columna NOT NULL | Baja | Medio | Migraciones con default primero, luego alter a NOT NULL |
| Timeout en migración large dataset | Baja | Bajo | Batch de migraciones grandes |

## Orden de migraciones

```
000001_create_extensions.up.sql           → CREATE EXTENSION vector, pgcrypto
000002_create_organizations.up.sql        → organizations
000003_create_users.up.sql                → users (FK → organizations)
000004_create_api_keys.up.sql             → api_keys (FK → users, organizations)
000005_create_projects.up.sql             → projects (FK → organization)
000006_create_observations.up.sql         → observations (FK → project) + vector + tsv
000007_create_sessions.up.sql             → sessions (FK → user)
000008_create_prompts.up.sql              → prompts (FK → project, user)
000009_create_knowledge_docs.up.sql       → knowledge_docs (FK → project)
000010_create_skills.up.sql               → skills (FK → organization)
000011_create_skill_versions.up.sql       → skill_versions (FK → skill)
000012_create_agents.up.sql               → agents (FK → organization)
000013_create_flows.up.sql                → flows (FK → organization)
000014_create_flow_runs.up.sql            → flow_runs (FK → flow, user)
000015_create_agent_runs.up.sql           → agent_runs (FK → agent, domain_flow_run)
000016_create_crons.up.sql                → crons (FK → organization)
000017_create_webhooks.up.sql             → webhooks (FK → organization)
000018_create_audit_log.up.sql            → audit_log (append-only, sin FK)
000019_create_secrets.up.sql              → secrets (FK → organization)
000020_create_cost_logs.up.sql            → cost_logs (FK → organization, domain_flow_run, domain_agent_run)
000021_create_project_templates.up.sql    → project_templates
000022_create_project_links.up.sql        → project_links (FK → projects)
000023_create_project_merges.up.sql       → project_merges (FK → projects)
```

Cada una con su correspondiente `.down.sql` que hace DROP TABLE IF EXISTS CASCADE.
