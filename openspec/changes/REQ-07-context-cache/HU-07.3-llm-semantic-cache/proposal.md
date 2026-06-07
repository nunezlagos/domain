# Proposal: HU-07.3-llm-semantic-cache

## Intención

Implementar un `SemanticCache` que almacene respuestas de LLM indexadas por embedding del prompt, y que permita cache hit cuando un nuevo prompt tiene similitud coseno >= threshold configurable.

## Scope

**In scope:**
- Módulo `SemanticCache` con métodos `Get(ctx, prompt) -> (response, hit)`, `Set(ctx, prompt, response)`, `Invalidate(ctx, prompt)`, `InvalidateByPattern(ctx, pattern)`
- Indexación por embedding vector (pgvector) con cosine similarity search
- TTL configurable por entrada (default: 60 min)
- Threshold de similitud configurable (default: 0.95)
- Límite de entradas en caché (LRU eviction cuando se excede)

**Out of scope:**
- Cache distribuido (solo local/in-process)
- Cache persistente en disco (solo PostgreSQL/pgvector)
- Cache de respuestas streaming

## Enfoque técnico

- Tabla `llm_cache` en PostgreSQL:
  ```sql
  CREATE TABLE llm_cache (
    prompt_hash TEXT PRIMARY KEY,
    prompt_text TEXT NOT NULL,
    embedding vector(1536),
    response TEXT NOT NULL,
    model TEXT NOT NULL,
    ttl TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '1 hour',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  ```
- `SemanticCache.Get()`: calcula embedding del prompt, busca en pgvector `ORDER BY embedding <=> $1 LIMIT 1`, si `distance < threshold` y `ttl > NOW()` → hit
- `SemanticCache.Set()`: almacena prompt, embedding, response, model, ttl
- LRU eviction: mantener contador de entradas, si excede límite, borrar las más viejas por `created_at`

## Riesgos

- Embedding computation por cada request agrega latencia (~100-300ms) pero es menor que una llamada LLM (~2-10s)
- PII en prompts cacheados → no cachear si prompt contiene datos sensibles (check por pattern)
- El threshold de similitud es crítico: muy bajo → respuestas incorrectas; muy alto → pocos hits. Default 0.95

## Testing

- **Unit:** Cache hit/miss logic, TTL expiration, invalidation
- **Integration:** Semantic cache + pgvector real
- **Gherkin:** Escenarios del hu.md
- **Sabotaje:** Bajar threshold a 0.1 → verificar que respuestas diferentes se consideran hit incorrectamente
