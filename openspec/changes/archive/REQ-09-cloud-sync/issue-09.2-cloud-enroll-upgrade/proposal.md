# Proposal: issue-09.2-cloud-enroll-upgrade

## Intención

Implementar el enrollment de instancias locales en el servidor cloud y el ciclo de vida de upgrade con subcomandos (doctor, repair, bootstrap, rollback, status). Una state machine gobierna las transiciones válidas entre estados.

## Scope

**Incluye:**
- `engram cloud enroll` — registro de instancia via POST /enroll
- `engram upgrade doctor` — checks de salud de la config cloud
- `engram upgrade repair` — auto-fix de issues detectables
- `engram upgrade bootstrap` — wizard interactivo de setup inicial
- `engram upgrade rollback` — restore de cloud.json.bak
- `engram upgrade status` — estado actual de la instancia
- State machine: none → configured → enrolled → upgraded (+ error)

**No incluye:**
- Server-side enrollment handling (issue-09.3)
- Dashboard UI (issue-09.4)
- Autosync (issue-09.5)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Enrollment | POST /api/enroll con {machine_id, hostname, version} → recibe {enrollment_id, server_version} |
| State machine | Enum con validaciones explícitas en cada transición |
| Backup | Copy cloud.json → cloud.json.bak antes de cualquier write |
| Machine ID | `/etc/machine-id` o `hostid` o hash de (hostname + OS) |
| Doctor checks | 4 checks: config válida, token valido (ping server), server reachable, enrollment activo |
| Bootstrap | Wizard: preguntar server → token → enroll → verify |

