# Tasks: issue-07.3-llm-semantic-cache

## Backend

- [x] Crear migración SQL para tabla `llm_cache` con pgvector index
- [x] Implementar `SemanticCache` struct con métodos `Get`, `Set`, `Invalidate`, `InvalidateByPattern`
- [x] Implementar `Get()`: compute embedding → pgvector search → verificar TTL → hit/miss
- [x] Implementar `Set()`: store prompt, embedding (pgvector), response, model, TTL
- [x] Implementar `Invalidate()` por prompt exacto y `InvalidateByPattern()` por LIKE/ILIKE
- [x] Implementar LRU eviction: límite de entradas (default 10k), eliminar más viejas por created_at
- [x] Configurar threshold de similitud (default 0.95) y TTL (default 60 min) vía config
- [x] Integrar con LLM provider: interceptar llamadas, check cache antes de llamar, store después
- [x] Agregar hook de pre-set: detectar PII → skip cache para ese entry

## Tests

- [x] Test unitario: Get exact match retorna response
- [x] Test unitario: Get semantic match (prompt similar > threshold) retorna cached
- [x] Test unitario: Get semantic miss (prompt diferente < threshold) llama LLM
- [x] Test unitario: TTL expirado = miss
- [x] Test unitario: Invalidate elimina entrada
- [x] Test unitario: LRU eviction elimina más vieja
- [x] Test de integración: SemanticCache + pgvector real con datos seeded
- [x] Test E2E: escenarios Gherkin del hu.md
- [x] Sabotaje: threshold 0.1 → prompts diferentes matchean → test falla

## Cierre

- [x] Verificación manual: latency comparison cache hit vs miss
- [x] Suite verde completa
- [x] Documentar threshold y TTL recomendados en config
