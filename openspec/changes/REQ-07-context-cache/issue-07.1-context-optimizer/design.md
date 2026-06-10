# Design: issue-07.1-context-optimizer

## Decisión arquitectónica

**Patrón:** Pipeline de procesamiento con stages encadenados.

```
ContextPool → Scorer → Sorter → Selector → Truncator → OptimizedContext
```

Cada stage implementa una interfaz simple y es reemplazable/testingable por separado.

- `ContextOptimizer` como facade del pipeline
- `ContextScorer` asigna score compuesto: `recent(0.5) + relevant(0.3) + structured(0.2)`
- `TruncationStrategy` se selecciona según config del modelo destino

**Persistence:** No se persisten resultados. Es cálculo en memoria por request.

## Alternativas descartadas

1. **LLM-based selection** (pedir al modelo que decida qué incluir): Descartado por costo y latencia. El propósito es precisamente ahorrar tokens.
2. **Sliding window fijo** (últimos N tokens siempre): Descartado porque no prioriza contenido relevante sobre contenido reciente pero trivial.
3. **Graph-based context** (navegar grafo de memorias para construir contexto): Demasiado complejo para MVP. Se puede agregar después.

## Diagrama

```
┌─────────────┐     ┌──────────┐     ┌────────┐     ┌───────────┐     ┌───────────┐
│ ContextPool │────▶│ Scorer   │────▶│ Sorter │────▶│ Selector  │────▶│ Truncator │
│             │     │          │     │        │     │ (by score)│     │ (if over) │
│ obs1 (500t) │     │ recent   │     │ desc   │     │           │     │           │
│ obs2 (300t) │     │ relevant │     │ score  │     │ picks up  │     │ middle or │
│ obs3 (1kt)  │     │ type     │     │        │     │ to budget │     │ tail      │
└─────────────┘     └──────────┘     └────────┘     └───────────┘     └───────────┘
```

## TDD plan

1. **Red:** Test que `Optimize()` con pool no vacío retorna <= budget tokens y scores en orden correcto
2. **Green:** Implementar pipeline mínimo
3. **Refactor:** Extraer interfaces por stage
4. **Sabotaje:** Poner scorer weights a 0 → solo primer stage gana; verificar falla

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|-----------|
| Token counting lento para fragmentos grandes | Cache LRU de token counts por content hash |
| Embedding similarity no disponible | Fallback a BM25 keyword scoring |
| Scoring weights inadecuados | Exponer como configuración por agente/flow |
| Truncamiento pierde información crítica | Preservar always-include markers (ej: system prompt nunca se trunca)
