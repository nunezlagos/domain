# Tasks: issue-07.3-llm-semantic-cache

## Backend

- [ ] Crear migración SQL para tabla `llm_cache` con pgvector index
- [ ] Implementar `SemanticCache` struct con métodos `Get`, `Set`, `Invalidate`, `InvalidateByPattern`
- [ ] Implementar `Get()`: compute embedding → pgvector search → verificar TTL → hit/miss
- [ ] Implementar `Set()`: store prompt, embedding (pgvector), response, model, TTL
- [ ] Implementar `Invalidate()` por prompt exacto y `InvalidateByPattern()` por LIKE/ILIKE
- [ ] Implementar LRU eviction: límite de entradas (default 10k), eliminar más viejas por created_at
- [ ] Configurar threshold de similitud (default 0.95) y TTL (default 60 min) vía config
- [ ] Integrar con LLM provider: interceptar llamadas, check cache antes de llamar, store después
- [ ] Agregar hook de pre-set: detectar PII → skip cache para ese entry

## Tests

- [ ] Test unitario: Get exact match retorna response
- [ ] Test unitario: Get semantic match (prompt similar > threshold) retorna cached
- [ ] Test unitario: Get semantic miss (prompt diferente < threshold) llama LLM
- [ ] Test unitario: TTL expirado = miss
- [ ] Test unitario: Invalidate elimina entrada
- [ ] Test unitario: LRU eviction elimina más vieja
- [ ] Test de integración: SemanticCache + pgvector real con datos seeded
- [ ] Test E2E: escenarios Gherkin del hu.md
- [ ] Sabotaje: threshold 0.1 → prompts diferentes matchean → test falla

## Cierre

- [ ] Verificación manual: latency comparison cache hit vs miss
- [ ] Suite verde completa
- [ ] Documentar threshold y TTL recomendados en config
