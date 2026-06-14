# Proposal: issue-07.1-context-optimizer

## Intención

Construir un ContextOptimizer que reciba un token budget y un pool de fragmentos de contexto, seleccione los más relevantes con prioridad recent > relevant > structured, y aplique truncamiento inteligente si se excede el límite.

## Scope

**In scope:**
- Módulo `ContextOptimizer` con método `Optimize(pool ContextPool, budget int) -> OptimizedContext`
- Algoritmo de scoring: recencia (timestamp) + relevancia (cosine similarity a query) + tipo (structured bonus)
- Estrategias de truncamiento: `truncate_middle` (preservar head + tail), `truncate_tail` (cortar desde el final)
- Integración con token counter (issue-06.6) para medición precisa
- Metadata en output: `total_tokens`, `truncated`, `items_selected`, `items_omitted`

**Out of scope:**
- UI de configuración de budgets
- Cache de resultados optimizados
- Persistencia de estrategias de optimización

## Enfoque técnico

- Interfaz `ContextOptimizer` con método `Optimize(ctx, input, budget)`
- `ContextScorer` que asigna un score compuesto: `w_recent * recency_score + w_relevant * relevance_score + w_structured * type_score`
- `TruncationStrategy` interface con implementaciones `TruncateMiddle` y `TruncateTail`
- Usar token counter de issue-06.6 para medir tokens exactos
- Pipeline: Score → Sort → Select within budget → Truncate if overflow → Return

## Riesgos

- Token counting puede ser caro para fragmentos grandes → cachear counts
- Embedding similarity requiere pgvector QUERY activo → fallback a BM25 si no disponible
- Scoring weights requieren tuning inicial → exponer como config

## Testing

- **Unit:** Scorer individual, truncation strategies, edge cases (budget = 0, pool vacía)
- **Integration:** Pipeline completo con token counter real
- **Gherkin:** Escenarios de selección y truncamiento del hu.md
- **Sabotaje:** Romper scorer weights → verificar que orden cambia
