# REQ-01-core-platform: Fundación del sistema: Postgres schema, migraciones versionadas, config por env, health checks, version info. Base de toda la plataforma.

**Estado:** activo
**Creado:** 2026-06-07

## Descripción

Fundación del sistema: Postgres schema, migraciones versionadas, config por env, health checks, version info. Base de toda la plataforma.

## Criterios de éxito

- Schema Postgres completo con 23 tablas, extensiones pgvector+pgcrypto, migraciones versionadas
- Sistema de configuración validado al startup con env DOMAIN_*
- Endpoint /health funcional con status, version, uptime y DB ping
- CLI `domain version` que reporta version/commit/build time
- Sistema de seeders idempotente con go:embed que pobla catálogos esenciales (plans, model registry, templates, policies, notification templates, error codes, system crons) al boot
- Platform policies (conventions, security rules, architecture, observability, migrations) persistidas en BD como source of truth, accesibles vía MCP tools, exportables a .claude/rules/*.md
- Catálogo de 10 personas (actores) persistido en BD con MCP tools, referenciado por cada HU vía field `**Persona:**` con enforcement por linter

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-01.1-db-schema-migrations | proposed | Schema Postgres + golang-migrate + pgvector + GIN + FKs |
| HU-01.2-config-system | proposed | Config desde env vars con validación y defaults |
| HU-01.3-health-version | proposed | GET /health + version embebida + CLI version |
| HU-01.4-project-templates | proposed | Templates de proyectos: skills default, scope, agentes y flows preconfigurados |
| HU-01.5-project-merge | proposed | Merge de proyectos, cross-project references, detección por git remote, relocate |
| HU-01.6-local-dev-environment | proposed | Docker Compose dev: Postgres+pgvector, MinIO (S3), Adminer, Mailpit, Makefile |
| HU-01.7-seeders-system | proposed | Framework Go seeders idempotente con go:embed: plans, model registry, templates, policies, error codes, crons |
| HU-01.8-platform-policies | proposed | Tabla platform_policies + CRUD + version + export/import md + MCP tools `domain_policy_get/search` |
| HU-01.9-personas-catalog | proposed | Catálogo de 10 personas (actores) en BD + MCP tools `domain_persona_get/list` + linter HU `**Persona:**` obligatorio + retrofit baseline |
