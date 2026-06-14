# Design: issue-23.2-soft-delete-restore

## Decisión arquitectónica

**Soft-delete pattern:** columna `deleted_at TIMESTAMPTZ NULL` (más simple que tabla separada).
**Filtering:** WHERE explícito en queries del store (no vistas — overhead innecesario).
**Cascade:** explícito en service, no trigger SQL (más visible en code review).
**Purge:** cron diario batch DELETE WHERE deleted_at < now() - interval.

## Schema diff

```sql
-- por cada entidad
ALTER TABLE observations ADD COLUMN deleted_at TIMESTAMPTZ;
CREATE INDEX ON observations (deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX ON observations (project_id, created_at) WHERE deleted_at IS NULL;
-- recrear índices del hot path con predicate WHERE deleted_at IS NULL
```

## Endpoints

| método | path | descripción |
|--------|------|-------------|
| GET | /trash | listar items en papelera (filtros entity_type, project_id) |
| GET | /trash/:entity_type/:id | detalle |
| POST | /trash/restore | restore (validation conflict) |

## Cron

```
purge-trash:
  schedule: "0 5 * * *"
  command: domain-mcp purge --older-than ${DOMAIN_TRASH_TTL_DAYS}d
```

## TDD plan

1. Soft-delete + listar trash + restore
2. Cascade soft-delete project → hijos
3. Restore conflict slug → 409
4. TTL purge: fixture viejo → cron purga
5. Linter SQL: query sin filtro → falla
6. Sabotaje: hard-delete antes de TTL → bloqueado salvo flag explícito
