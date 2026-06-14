# Tasks: issue-07.1-context-optimizer

## Backend

- [x] Implementar interfaz `ContextOptimizer` con método `Optimize(ctx, input ContextPool, budget int)`
- [x] Implementar `ContextScorer` con scoring compuesto: recencia + relevancia + tipo
- [x] Implementar `TruncationStrategy` interface con `TruncateMiddle` y `TruncateTail`
- [x] Implementar pipeline completo: Score → Sort → Select → Truncate
- [x] Integrar con token counter (issue-06.6) para medición precisa de tokens
- [x] Integrar con embedding similarity (issue-06.5) para scoring de relevancia
- [x] Agregar metadata en output: total_tokens, truncated, items_selected, items_omitted
- [x] Exponer configuración de weights (recent, relevant, structured) vía config del agente

## Tests

- [x] Test unitario: ContextScorer con datos mock
- [x] Test unitario: TruncateMiddle preserva head + tail
- [x] Test unitario: TruncateTail corta desde el final
- [x] Test unitario: budget exacto no modifica input
- [x] Test unitario: pool vacío retorna vacío sin error
- [x] Test unitario: tiebreaker por ID cuando timestamps iguales
- [x] Test de integración: pipeline completo con token counter real
- [x] Test E2E: escenarios Gherkin del hu.md
- [x] Sabotaje: scorer weights en 0 → verificar pipeline falla

## Cierre

- [x] Verificación manual con datos reales de memoria
- [x] Suite verde completa
- [x] Documentar weights recomendados en AGENTS.md o config
