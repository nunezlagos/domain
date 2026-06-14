# Proposal: issue-05.4-auto-skill-engine

## Intención

Motor de recomendación automática de skills. Dado un contexto textual (descripción de la tarea del agente), el sistema genera un embedding de ese contexto y encuentra los skills más cercanos semánticamente usando pgvector. Esto permite que los agentes descubran skills relevantes sin intervención manual.

## Scope

**Incluye:**
- Endpoint POST /api/skills/recommend: recomendación single context
- Endpoint POST /api/skills/recommend/batch: recomendación multi-context
- Generación de embedding del contexto vía provider de embeddings (issue-06.5)
- Búsqueda por similitud coseno en pgvector
- Filtros: type_filter, project_id, exclude_project, tags_filter
- Threshold mínimo de relevancia
- Top-N configurable (default 5, max 20)
- Timeout configurable para embedding generation
- Cache de embeddings de contexto (TTL 5 min) para contextos repetidos

**Excluye:**
- Re-ranking con LLM (futuro)
- Feedback loop (aprender de qué skills se usaron realmente)
- Recomendación basada en historial de uso

## Enfoque técnico

- El contexto se embeddea usando el mismo provider de issue-06.5.
- Query pgvector: `SELECT *, 1 - (embedding <=> $emb) AS score FROM skills WHERE embedding IS NOT NULL AND score >= $threshold ORDER BY score DESC LIMIT $top_n`.
- Filtros se agregan como WHERE clauses adicionales.
- Cache LRU de embeddings de contexto con TTL para evitar re-embedder el mismo texto repetido.
- Timeout con context.WithTimeout(5s) para la generación de embedding.

## Riesgos

- **Performance:** Calcular similitud contra 100K skills puede ser lento. Mitigación: IVFFlat index con probes sintonizables.
- **Embedding provider caído:** El motor no puede funcionar. Mitigación: cache, retry, error message claro.
- **Threshold muy bajo:** Skills irrelevantes. Default threshold = 0.5.
- **Cache de contexto:** Si el contexto es siempre único, el cache no ayuda. Aún así, el embedding individual es rápido (< 1s).

## Testing

- **Unitarios:** Generación de query con filtros, ordenamiento por score, threshold filtering.
- **Integración:** Seed de skills con embeddings, recomendar con contextos, verificar scores y orden.
- **E2E:** Contexto real → skills relevantes.
- **Sabotaje:** Embedding provider timeout → 503 graceful.
