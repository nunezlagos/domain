# Design: HU-05.2-skill-registry-search

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativa | Razón |
|----------|---------------|-------------|-------|
| FTS engine | PostgreSQL tsvector | ElasticSearch | Sin dependencia externa, mismo DB |
| Semantic search | pgvector <=> (cosine distance) | In-process embedding compare | Aprovecha índices de pgvector |
| Hybrid mode | Weighted sum de scores normalizados | Two-phase rerank | Simple, efectivo, rápido |
| Index type | IVFFlat | HNSW | Menor consumo de memoria, tuning más simple |
| Dictionary | `simple` | `spanish` o `english` | Independencia del idioma |

## Alternativas descartadas

- **ElasticSearch/Searchify:** Overkill para el volumen esperado, añade dependencia operativa.
- **Búsqueda solo semántica:** La FTS es necesaria para queries exactas (IDs, slugs, nombres técnicos).
- **HNSW:** Mejor recall pero más memoria. IVFFlat es suficiente para < 100K skills.

## Diagrama

```
POST /api/skills/search { query, mode, top_k, threshold, filters }
  │
  ├─► mode = "fts"
  │     └─► SELECT *, ts_rank(search_vector, query) AS score
  │           FROM skills WHERE search_vector @@ plainto_tsquery('simple', $1)
  │           AND filters... ORDER BY score DESC LIMIT top_k
  │
  ├─► mode = "semantic"
  │     ├─► Generar embedding de query vía provider (HU-06.5)
  │     └─► SELECT *, 1 - (embedding <=> $emb) AS score
  │           FROM skills WHERE embedding IS NOT NULL
  │           AND (1 - (embedding <=> $emb)) >= threshold
  │           AND filters... ORDER BY score DESC LIMIT top_k
  │
  └─► mode = "hybrid"
        ├─► FTS query → scores normalizados [0,1]
        ├─► Semantic query → scores normalizados [0,1]
        └─► Combined = (fts_score * fts_weight) + (sem_score * sem_weight)
              ORDER BY combined DESC LIMIT top_k
```

### Modelo extendido

```sql
-- Añadir columna tsvector a skills
ALTER TABLE skills ADD COLUMN search_vector TSVECTOR
    GENERATED ALWAYS AS (
        to_tsvector('simple', COALESCE(name, '') || ' ' || COALESCE(description, ''))
    ) STORED;

CREATE INDEX idx_skills_search_vector ON skills USING GIN(search_vector);
```

## TDD plan

1. **TestFTSBasico:** Insertar skill "Generar resumen", buscar "resumen" → 1 resultado
2. **TestFTSSinResultados:** Buscar texto inexistente → 0 resultados
3. **TestFTSMultiPalabra:** Buscar "resumen ejecutivo" → encuentra skills con ambas palabras
4. **TestSemantico:** Query semántica devuelve resultados ordenados por score
5. **TestSemanticoConThreshold:** Query con threshold alto filtra resultados bajos
6. **TestSemanticoEmbeddingNull:** Skills sin embedding no aparecen
7. **TestHybrido:** Combinación de FTS + semántico con pesos
8. **TestFiltroType:** type=prompt solo devuelve prompts
9. **TestFiltroTags:** tags=resumen devuelve skills con ese tag
10. **TestFiltrosCombinados:** type=prompt + tags=resumen + project_id filter
11. **TestPaginacion:** limit=5 offset=0 devuelve 5 resultados con total correcto
12. **TestSabotajeSinEmbeddingProvider:** Búsqueda semántica devuelve error claro

## Riesgos y mitigación

- **IVFFlat tuning:** El召回 depende de listas y probes. Documentar parámetros y proveer defaults.
- **Embedding caído:** Búsqueda semántica no puede operar. Devolver error 503 con mensaje claro.
- **TSVECTOR no actualizado:** Usar GENERATED ALWAYS para consistencia.
