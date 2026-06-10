# Tasks: issue-04.4-mcp-session-tools

## Backend

- [ ] Definir modelo `Session` en `internal/engram/session.go`: ID, Directory, Status, StartedAt, EndedAt, HasSummary
- [ ] Crear tabla `sessions` en SQLite schema
- [ ] Implementar `SessionStore` con métodos: Create, Get, Update, List
- [ ] Implementar `domain_mem_session_start`: validar/auto-generar id, idempotente
- [ ] Implementar `domain_mem_session_end`: validar existencia y estado activo, guardar summary si se provee
- [ ] Implementar `domain_mem_session_summary`: parser markdown de secciones (## Accomplished, ## Decisions, ## Next Steps)
- [ ] Implementar creación de observaciones hijas desde items parseados
- [ ] Implementar `domain_mem_capture_passive`: heurística de tipo (pattern/context), vincular a session
- [ ] Implementar generación de UUID v4 automática si no se provee session id
- [ ] Integrar con server.go: registrar 4 handlers

## Tests

- [ ] Test unitario: `TestSessionStart`, `TestSessionStartIdempotent`
- [ ] Test unitario: `TestSessionEnd`, `TestSessionEndNotFound`, `TestSessionEndAlreadyEnded`
- [ ] Test unitario: `TestSessionEndWithSummary`
- [ ] Test unitario: `TestSessionSummary`, `TestSessionSummaryNoSections`
- [ ] Test unitario: `TestCapturePassive`, `TestCapturePassiveWithSession`, `TestCapturePassiveEmpty`
- [ ] Test unitario: `TestMarkdownParser` varias combinaciones de secciones
- [ ] Test integración: secuencia start → capture → end → summary → end fail
- [ ] Sabotaje: session_end sin start previo → error. markdown `# Accomplished` → no parsea.

## Cierre

- [ ] Verificación manual: ciclo completo start → capture → end con resumen
- [ ] Suite verde: `go test ./internal/...`
