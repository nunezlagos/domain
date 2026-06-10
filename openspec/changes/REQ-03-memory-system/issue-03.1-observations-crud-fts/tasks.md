# Tasks: issue-03.1-observations-crud-fts

## Backend

- [ ] `migrations/XXXX_create_observations.sql`: tabla (incluye created_by FK → users(id)) + generated column + índices GIN/BTREE
- [ ] `internal/store/pg/observation.go`: interfaz `ObservationStore` + structs (`Observation`, `ObservationSearchResult`, `ObservationFilter`)
- [ ] `internal/store/pg/observation_store.go`: implementación con sqlx, queries parametrizadas
- [ ] Implementar `Insert` con generated column automática (no incluir tsv en INSERT)
- [ ] Implementar `GetByID`, `Update`, `Delete`
- [ ] Implementar `Search` con `ts_rank` + `ts_headline` + `plainto_tsquery`
- [ ] Implementar filtros combinables (type, project_id, scope, limit) dinámicos con `WHERE` condicional
- [ ] Implementar `FindConflicts` para conflict detection (ts_rank > 0.7, top 3)
- [ ] `internal/memory/service.go`: orquestar llamadas a store, aplicar conflict detection antes de insert

## Tests

- [ ] Test unitario de `MemoryService` con store mockeado
- [ ] Test de integración: insert y search con container Postgres
- [ ] Test de tsvector generated column (verificar que se genera correctamente)
- [ ] Test de conflict detection: insertar duplicado → obtener candidatos
- [ ] Test de filtros combinados (type + project_id + scope + limit)
- [ ] Test de búsqueda con caracteres especiales y unicode
- [ ] Test de búsqueda con empty string (debe devolver error o empty)
- [ ] Test de ranking: contenido más relevante primero
- [ ] Sabotaje: dropear índice GIN → search debe fallar ruidosamente

## Cierre

- [ ] Verificación manual: insertar 3 observaciones, buscar por palabra clave, confirmar ranking
- [ ] Suite verde
