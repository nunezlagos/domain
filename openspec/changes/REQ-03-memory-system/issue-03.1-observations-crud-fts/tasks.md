# Tasks: issue-03.1-observations-crud-fts

## Backend

- [x] `migrations/XXXX_create_observations.sql`: tabla (incluye created_by FK → users(id)) + generated column + índices GIN/BTREE
- [x] `internal/store/pg/observation.go`: interfaz `ObservationStore` + structs (`Observation`, `ObservationSearchResult`, `ObservationFilter`)
- [x] `internal/store/pg/observation_store.go`: implementación con sqlx, queries parametrizadas
- [x] Implementar `Insert` con generated column automática (no incluir tsv en INSERT)
- [x] Implementar `GetByID`, `Update`, `Delete`
- [x] Implementar `Search` con `ts_rank` + `ts_headline` + `plainto_tsquery`
- [x] Implementar filtros combinables (type, project_id, scope, limit) dinámicos con `WHERE` condicional
- [x] Implementar `FindConflicts` para conflict detection (ts_rank > 0.7, top 3)
- [x] `internal/memory/service.go`: orquestar llamadas a store, aplicar conflict detection antes de insert

## Tests

- [x] Test unitario de `MemoryService` con store mockeado
- [x] Test de integración: insert y search con container Postgres
- [x] Test de tsvector generated column (verificar que se genera correctamente)
- [x] Test de conflict detection: insertar duplicado → obtener candidatos
- [x] Test de filtros combinados (type + project_id + scope + limit)
- [x] Test de búsqueda con caracteres especiales y unicode
- [x] Test de búsqueda con empty string (debe devolver error o empty)
- [x] Test de ranking: contenido más relevante primero
- [x] Sabotaje: dropear índice GIN → search debe fallar ruidosamente

## Cierre

- [x] Verificación manual: insertar 3 observaciones, buscar por palabra clave, confirmar ranking
- [x] Suite verde
