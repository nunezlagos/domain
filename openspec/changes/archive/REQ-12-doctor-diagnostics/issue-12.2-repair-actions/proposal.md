# Proposal: issue-12.2-repair-actions

## Intención

Extender `engram doctor` con modo `--repair` que corrige automáticamente issues detectables (directorios faltantes, canonicalización de proyectos, sesiones abiertas antiguas, orphans). Incluye dry-run, límite de acciones, e idempotencia.

## Scope

**Incluye:**
- `engram doctor --repair [--dry-run] [--fix-orphans] [--max-actions=N]`
- Repair actions: create missing dirs, normalize projects, close stale sessions, soft-delete orphans
- RepairPlan: lista de acciones propuestas
- Idempotencia: cada acción verifica si ya fue aplicada
- Reporte de acciones tomadas/fallidas

**No incluye:**
- Repair de sync state (issue-12.1 solo detecta)
- Repair de DB corruption (PRAGMA integrity_check es read-only)
- Migración de datos compleja (issue-08.3)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Repair actions | Slice de `RepairAction` con Execute() y Validate() methods |
| Dry-run | Solo generar RepairPlan sin ejecutar |
| Idempotencia | Cada acción checkea precondición antes de ejecutar |
| Límite | --max-actions trunca RepairPlan; reporta remaining |
| Transaccional | DB actions en transacción; filesystem actions no rollbackeables |

