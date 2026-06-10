# Design: issue-05.4-auto-skill-engine

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativa | Razón |
|----------|---------------|-------------|-------|
| Similarity search | pgvector <=> (cosine) | In-memory | Aprovecha índice DB, escala |
| Cache context embedding | LRU + TTL (5 min) | Sin cache | Reduce llamadas a LLM provider |
| Batch mode | Parallel goroutines | Sequential | Velocidad en batch |
| Default threshold | 0.5 | 0.7 | Balance recall/precision |
| Top-N max | 20 | Sin límite | Protección contra respuestas enormes |

## Alternativas descartadas

- **In-memory similarity:** No escala. Con pgvector el índice maneja 100K+ vectores eficientemente.
- **Sin cache:** Para workflows batch con contextos repetitivos, el cache reduce latencia 50-80%.
- **Re-ranking con LLM:** Costoso y lento. Dejamos para fase futura.

## Diagrama

```
POST /api/skills/recommend { context, top_n, threshold, filters }
  │
  ├─► Validar: context no vacío, top_n <= 20
  ├─► Cache hit? → usar embedding cacheado
  │     └─► Cache miss → generar embedding vía provider (timeout 5s)
  │           └─► Timeout? → 503
  ├─► Query pgvector con filtros:
  │     SELECT id, name, slug, description, type, tags,
  │            1 - (embedding <=> $emb) AS score
  │     FROM skills
  │     WHERE embedding IS NOT NULL
  │       AND (1 - (embedding <=> $emb)) >= $threshold
  │       [AND type = $type_filter]
  │       [AND project_id != $exclude_project]
  │       [AND tags && $tags_filter]
  │     ORDER BY score DESC
  │     LIMIT $top_n
  ├─► Cachear embedding (si no estaba)
  └─► 200 OK { data: [...], meta: { total, threshold, context_truncated } }
```

### Cache LRU

```
type ContextEmbeddingCache struct {
    mu       sync.RWMutex
    cache    *lru.Cache[string, cachedEmbedding]
    ttl      time.Duration  // 5 min
}

type cachedEmbedding struct {
    embedding []float32
    expiresAt time.Time
}
```

## TDD plan

1. **TestRecommendBasico:** Contexto → skills ordenados por score
2. **TestRecommendConThreshold:** Filtra skills por debajo de threshold
3. **TestRecommendConTypeFilter:** Solo devuelve type especificado
4. **TestRecommendExcludeProject:** Excluye proyecto indicado
5. **TestRecommendTopN:** Respeta top_n máximo
6. **TestRecommendContextoVacio:** 400 Bad Request
7. **TestRecommendSinResultados:** Array vacío + message
8. **TestRecommendCache:** Misma contexto 2 veces → 2da llamada usa cache (más rápida)
9. **TestBatchRecommend:** Batch con 3 contextos → 3 grupos de resultados
10. **TestEmbeddingTimeout:** Provider tarda > 5s → 503
11. **TestSabotaje:** Provider da error → 503 con mensaje claro

## Riesgos y mitigación

- **Cache invalidation:** Si se añaden nuevos skills, el cache de embedding de contexto stale no afecta (la query sigue siendo correcta, solo evitamos re-embedder).
- **Threshold muy bajo:** Skills irrelevantes. Default 0.5 es conservador.
- **Contexto muy largo:** Truncar a 512 tokens antes de embedder para evitar costos altos.
