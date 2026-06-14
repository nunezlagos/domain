# Proposal: issue-10.3-conflict-cli-api

## Intención

Proveer interfaz CLI completa para gestión de conflictos y endpoints HTTP para integración. Incluye manejo de deferred sync queue con capacidad de replay.

## Scope

**Incluye:**
- CLI: `engram conflicts list|show|stats|scan|deferred [subcommands]`
- HTTP: GET /conflicts, GET /conflicts/:id, POST /conflicts/:id/judge, POST /conflicts/scan
- Deferred queue: list, show, replay
- Shared logic layer entre CLI y HTTP

**No incluye:**
- Conflict search annotations (issue-10.4)
- Semantic judge logic (issue-10.2)
- Lexical scan logic (issue-10.1)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| CLI | Cobra commands dentro de `engram conflicts` |
| HTTP | chi/subrouter en /conflicts |
| Shared logic | Internal service functions llamadas por CLI y HTTP |
| Deferred | sync_apply_deferred CRUD + replay function |
| Output CLI | table format (default) + --json flag |

