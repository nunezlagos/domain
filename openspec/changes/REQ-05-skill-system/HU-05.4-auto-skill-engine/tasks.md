# Tasks: HU-05.4-auto-skill-engine

## Backend

- [ ] Implementar endpoint POST /api/skills/recommend
- [ ] Implementar endpoint POST /api/skills/recommend/batch
- [ ] Implementar generación de embedding para contexto con timeout
- [ ] Implementar query pgvector con filtros combinados
- [ ] Implementar cache LRU de embeddings de contexto con TTL
- [ ] Implementar truncado de contexto (512 tokens) antes de embedder
- [ ] Implementar validaciones: contexto requerido, top_n max 20
- [ ] Implementar respuesta con scores de relevancia y metadata

## Frontend

- [ ] N/A (solo API)

## Tests

- [ ] Test unitario: query builder con filtros
- [ ] Test unitario: cache LRU hit/miss/expiry
- [ ] Test unitario: truncado de contexto largo
- [ ] Test integración: recomendación con datos seed
- [ ] Test integración: batch recomendación
- [ ] Test integración: threshold filtra correctamente
- [ ] Test integración: type filter y exclude project
- [ ] Sabotaje: embedding provider timeout → 503 graceful

## Cierre

- [ ] Verificación manual con curl
- [ ] Suite verde
