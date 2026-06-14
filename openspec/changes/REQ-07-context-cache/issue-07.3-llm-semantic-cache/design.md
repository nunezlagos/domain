# Design: issue-07.3-llm-semantic-cache

## Decisión arquitectónica

**Patrón:** Cache-aside (lectura: check cache → miss → compute → store → return).

```
Get(prompt) ──▶ compute embedding ──▶ pgvector search
                   │                        │
                hit ◀──── distance < 0.05? ──┘
                   │
                   ▼
              return cached response
                                       
                   │ miss
                   ▼
              call LLM ──▶ store(prompt, response) ──▶ return
```

- Embedding se computa en cada `Get()` para mantener consistencia semántica
- pgvector search con `<=>` (cosine distance) indexado con IVFFlat
- TTL se verifica en la query SQL: `WHERE ttl > NOW()`

## Alternativas descartadas

1. **Redis con search modules:** Más rápido pero agrega dependencia externa. Ya tenemos PostgreSQL con pgvector.
2. **Exact match cache solamente:** No cubre el caso de uso semántico. Prompt "Hola" vs "Hola!" serían miss.
3. **Cache en memoria (map/sync.Map):** No escala, no persiste entre reinicios, no permite búsqueda vectorial.

## Diagrama

```
┌──────────┐     ┌──────────────┐     ┌────────────┐     ┌───────────┐
│ Request  │────▶│ SemanticCache│────▶│ Embeddings │────▶│ pgvector  │
│ prompt   │     │              │     │ (issue-06.5)  │     │           │
└──────────┘     │ .Get(prompt) │     └────────────┘     │ llm_cache│
                 │ .Set(prompt) │                        └───────────┘
                 │ .Invalidate()│                              │
                 └──────────────┘                              ▼
                        │                               SELECT * FROM llm_cache
                        ▼                               WHERE ttl > NOW()
                   ┌──────────┐                          ORDER BY embedding <=> $1
                   │ LLM Call │                          LIMIT 1
                   │ (on miss)│
                   └──────────┘
```

## TDD plan

1. **Red:** Test `Get()` con prompt exacto retorna response previa (Set + Get)
2. **Green:** Implementar Set (store embedding + response) y Get (exact match first)
3. **Refactor:** Agregar semantic search con pgvector
4. **Sabotaje:** Bajar threshold a 0.1 → prompts diferentes matchean → test debe fallar

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|-----------|
| Embedding call por cada Get agrega latencia | Cachear embedding en memoria LRU (hash → embedding), recalcular solo si no está en cache local |
| Threshold bajo da respuestas incorrectas | Default 0.95; exponer por modelo; loggear hits con distancia para monitoreo |
| PII en prompts cacheados | Hook de pre-set: si prompt matchea patterns PII, no cachear |
| pgvector index se degrada con muchas escrituras | IVFFlat con rebuild programado; limitar tamaño de cache a 10k entries
