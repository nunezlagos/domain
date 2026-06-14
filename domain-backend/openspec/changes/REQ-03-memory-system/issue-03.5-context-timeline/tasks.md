# Tasks: issue-03.5-context-timeline

## Backend

- [x] `internal/memory/context.go`: structs `ContextResult`, `TimelineResult`, `TimelineEntry`
- [x] Implementar `GetContext(project, scope string) (ContextResult, error)` con errgroup paralelo
- [x] Implementar `GetTimeline(observationID uuid.UUID, before, after int) (TimelineResult, error)`
- [x] `internal/memory/context_format.go`: `FormatContext(ctx ContextResult) string`
- [x] `internal/memory/timeline_format.go`: `FormatTimeline(tl TimelineResult) string`
- [x] Agregar método `GetLastSessions` a SessionStore (ya existe en issue-03.2)
- [x] Agregar método `GetLastObservations` a ObservationStore
- [x] Agregar método `GetLastPrompts` a PromptStore
- [x] Scope filtering: lógica para determinar qué scopes incluir según el scope solicitado

## Tests

- [x] Test unitario de formateo: contexto con datos → formato correcto
- [x] Test unitario de formateo: contexto vacío → "No active session" / "No recent entries"
- [x] Test de integración: insertar sesión + observación + prompt → GetContext → verificar todo presente
- [x] Test de integración: timeline con observación en posición media
- [x] Test de integración: timeline con observación más antigua (0 before)
- [x] Test de scope filtering: scope=project excluye personales y globales
- [x] Sabotaje: sin índices en created_at → query funciona pero más lenta

## Cierre

- [x] Verificación manual: ejecutar GetContext y verificar salida formateada legible
- [x] Suite verde
