# Tasks: HU-05.2-skill-registry-search

## Backend

- [ ] Crear migración para columna `search_vector` + índice GIN
- [ ] Implementar función de query FTS con tsquery y ranking
- [ ] Implementar función de query semántica con pgvector <=>
- [ ] Implementar generación de embedding para query en búsqueda semántica
- [ ] Implementar combinación híbrida de scores con pesos
- [ ] Implementar handler GET /api/skills con filtros (type, project_id, tags) y paginación
- [ ] Implementar handler POST /api/skills/search con modos fts/semantic/hybrid
- [ ] Implementar filtros combinables en ambas búsquedas
- [ ] Implementar threshold mínimo para resultados semánticos

## Frontend

- [ ] N/A (solo API)

## Tests

- [ ] Test unitario: query FTS genera WHERE clause correcta
- [ ] Test unitario: combinación híbrida de scores
- [ ] Test unitario: filtros se concatenan correctamente
- [ ] Test integración: búsqueda FTS sobre datos seed
- [ ] Test integración: búsqueda semántica devuelve scores
- [ ] Test integración: modo híbrido con pesos distintos
- [ ] Test integración: paginación devuelve total correcto
- [ ] Sabotaje: skills sin embedding excluidos de búsqueda semántica

## Cierre

- [ ] Verificación manual con curl
- [ ] Suite verde
