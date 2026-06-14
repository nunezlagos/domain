# Tasks: issue-05.2-skill-registry-search

## Backend

- [x] Crear migración para columna `search_vector` + índice GIN
- [x] Implementar función de query FTS con tsquery y ranking
- [x] Implementar función de query semántica con pgvector <=>
- [x] Implementar generación de embedding para query en búsqueda semántica
- [x] Implementar combinación híbrida de scores con pesos
- [x] Implementar handler GET /api/skills con filtros (type, project_id, tags) y paginación
- [x] Implementar handler POST /api/skills/search con modos fts/semantic/hybrid
- [x] Implementar filtros combinables en ambas búsquedas
- [x] Implementar threshold mínimo para resultados semánticos

## Frontend

- [x] N/A (solo API)

## Tests

- [x] Test unitario: query FTS genera WHERE clause correcta
- [x] Test unitario: combinación híbrida de scores
- [x] Test unitario: filtros se concatenan correctamente
- [x] Test integración: búsqueda FTS sobre datos seed
- [x] Test integración: búsqueda semántica devuelve scores
- [x] Test integración: modo híbrido con pesos distintos
- [x] Test integración: paginación devuelve total correcto
- [x] Sabotaje: skills sin embedding excluidos de búsqueda semántica

## Cierre

- [x] Verificación manual con curl
- [x] Suite verde
