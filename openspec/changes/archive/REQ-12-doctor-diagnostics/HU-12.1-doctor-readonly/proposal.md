# Proposal: HU-12.1-doctor-readonly

## Intención

Implementar `engram doctor`, un comando de diagnóstico read-only que ejecuta checks organizados en categorías (project health, session integrity, sync state, DB integrity) y produce un reporte estructurado en JSON.

## Scope

**Incluye:**
- `engram doctor [--json]` CLI command
- 4 categorías de checks: project, sessions, sync, db
- PRAGMA integrity_check en SQLite
- Reporte JSON estructurado con timestamp, version, duration
- Timeout individual por check (5s)
- Read-only garantizado (solo SELECT, PRAGMA, stat)

**No incluye:**
- Repair actions (HU-12.2)
- Health endpoint (HU-12.3)
- Auto-fix de issues

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Check interface | `Check(ctx) Result` con Name, Status, Message, Duration |
| Categories | Slice de slices; cada categoría es un grupo de checks |
| Timeout | context.WithTimeout por check individual |
| Report | Struct DoctorReport con envelope + results |
| Read-only | Solo SELECT y PRAGMA; ningún INSERT/UPDATE/DELETE |
| JSON output | `--json` flag; default output es table |

