# Proposal: issue-08.3-consolidation-migration

## Intención

Permitir consolidar datos de un proyecto origen en un proyecto destino, migrando todas las observaciones, sesiones y prompts asociados. Incluye endpoint HTTP, CLI interactivo, tool function para agentes, y dry-run mode para previsualización.

## Scope

**Incluye:**
- `ConsolidateProjects(ctx, db, from, to string, opts ConsolidateOpts) (*ConsolidateResult, error)`
- Dry-run mode (solo cuenta, no escribe)
- Transacción única para consistencia
- POST /projects/migrate endpoint
- `engram projects consolidate` CLI command con --interactive y --dry-run
- `mem_merge_projects` tool function

**No incluye:**
- Detección automática de duplicados (issue-08.2 solo alerta)
- Eliminación física del proyecto origen (los datos se reasignan, no se borran)
- Migración de config files o git remotes

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Migración DB | `UPDATE sessions SET project = ? WHERE project = ?` + mismo para observations |
| Transaccional | Begin/Commit/Rollback explícito; si algo falla, rollback completo |
| Dry-run | SELECT COUNT(*) en lugar de UPDATE; retorna estimaciones |
| CLI | `engram projects consolidate --from X --to Y [--dry-run] [--interactive]` |
| HTTP | POST /api/projects/migrate con JSON body {from, to, dry_run} |
| Tool function | `mem_merge_projects(from, to string) -> {ok, migrated}` |

