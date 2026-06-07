# Tasks: HU-03.5-context-timeline

## Backend

- [ ] `internal/memory/context.go`: structs `ContextResult`, `TimelineResult`, `TimelineEntry`
- [ ] Implementar `GetContext(project, scope string) (ContextResult, error)` con errgroup paralelo
- [ ] Implementar `GetTimeline(observationID uuid.UUID, before, after int) (TimelineResult, error)`
- [ ] `internal/memory/context_format.go`: `FormatContext(ctx ContextResult) string`
- [ ] `internal/memory/timeline_format.go`: `FormatTimeline(tl TimelineResult) string`
- [ ] Agregar método `GetLastSessions` a SessionStore (ya existe en HU-03.2)
- [ ] Agregar método `GetLastObservations` a ObservationStore
- [ ] Agregar método `GetLastPrompts` a PromptStore
- [ ] Scope filtering: lógica para determinar qué scopes incluir según el scope solicitado

## Tests

- [ ] Test unitario de formateo: contexto con datos → formato correcto
- [ ] Test unitario de formateo: contexto vacío → "No active session" / "No recent entries"
- [ ] Test de integración: insertar sesión + observación + prompt → GetContext → verificar todo presente
- [ ] Test de integración: timeline con observación en posición media
- [ ] Test de integración: timeline con observación más antigua (0 before)
- [ ] Test de scope filtering: scope=project excluye personales y globales
- [ ] Sabotaje: sin índices en created_at → query funciona pero más lenta

## Cierre

- [ ] Verificación manual: ejecutar GetContext y verificar salida formateada legible
- [ ] Suite verde
