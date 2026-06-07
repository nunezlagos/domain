# Proposal: HU-23.2-soft-delete-restore

## Intención

Aplicar patrón soft-delete uniforme a todas las entidades del dominio: `deleted_at TIMESTAMPTZ`, vistas filtradas por defecto, endpoints de papelera y restore, cron purge con TTL configurable, cascade consistente.

## Scope

**Incluye:**
- Migración agregando `deleted_at TIMESTAMPTZ NULL` a entidades: organizations, users, projects, observations, sessions, prompts, knowledge_docs, skills, agents, flows
- Vistas `*_active` o WHERE deleted_at IS NULL en todas las queries del repo
- Endpoint GET /trash con filtros
- Endpoint POST /trash/restore
- Cron diario `0 5 * * *` que hard-deletes lo expirado y borra S3 attachments
- Cascade: trigger SQL o lógica en service que marca hijos al soft-delete del padre

**No incluye:**
- Undo multi-step (un solo restore por entidad)
- Versioning de cambios pre-delete (audit_log ya tiene)

## Enfoque técnico

1. Trigger Postgres `BEFORE DELETE` que en lugar de DELETE setea deleted_at (alternativa: hacer en service)
2. Vistas materializadas a evaluar para hot paths
3. Soft-delete cascade vía service explícito (no FK ON DELETE)
4. Hard-delete cron usa batch + LIMIT para no lockear

## Riesgos

- Migración masiva: muchos índices a recrear con WHERE deleted_at IS NULL → done en batches
- Queries existentes sin filtro deleted_at → linter SQL/code review
- Cron purge irreversible → confirm con audit + double-check

## Testing

- Soft-delete + restore happy path
- Cascade restore
- Conflict slug en restore
- TTL purge: insertar deleted_at antiguo → cron purga
- Linter: query sin filtro deleted_at → falla
