# Tasks: HU-04.3-mcp-search-tools

## Backend

- [ ] Crear `internal/mcp/tools/search.go` con los 5 tool handlers de lectura
- [ ] Implementar `domain_mem_search`: construir query FTS5 con filtros opcionales (type, project, scope)
- [ ] Implementar FTS5 query builder: sanitizar input, armar WHERE dinámico
- [ ] Implementar paginación con limit (default 20, max 100)
- [ ] Implementar truncado de contenido a 500 chars en search results
- [ ] Implementar anotación de conflictos: JOIN con tabla de conflictos pendientes
- [ ] Implementar `domain_mem_context`: últimas 10 obs + sesión activa + metadata
- [ ] Implementar `domain_mem_timeline`: before/after queries con límites configurables
- [ ] Implementar `domain_mem_get_observation`: SELECT sin truncamiento
- [ ] Implementar `domain_mem_stats`: queries de agregación (COUNT, MIN, MAX, SUM)
- [ ] Asegurar que store usa SQLite :memory: con FTS5 habilitado para tests
- [ ] Integrar con server.go: registrar 5 handlers

## Tests

- [ ] Test unitario: `TestMemSearchBasic`, `TestMemSearchEmpty`, `TestMemSearchFilterType`
- [ ] Test unitario: `TestMemSearchFilterProject`, `TestMemSearchAllProjects`
- [ ] Test unitario: `TestMemSearchLimit`, `TestMemSearchConflictAnnotation`
- [ ] Test unitario: `TestMemContext`, `TestMemTimeline`, `TestMemTimelineEdge`
- [ ] Test unitario: `TestMemGetObservation`, `TestMemGetObservationNotFound`
- [ ] Test unitario: `TestMemStats`
- [ ] Test integración: insertar 50 obs, search por múltiples términos, verificar rankings
- [ ] Sabotaje: query FTS5 con caracteres especiales `" OR "1"="1` → no SQL injection, no crash

## Cierre

- [ ] Verificación manual: search por término conocido, verificar resultados y anotaciones
- [ ] Suite verde: `go test ./internal/mcp/...`
