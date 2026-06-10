# Tasks: issue-07.1-context-optimizer

## Backend

- [ ] Implementar interfaz `ContextOptimizer` con método `Optimize(ctx, input ContextPool, budget int)`
- [ ] Implementar `ContextScorer` con scoring compuesto: recencia + relevancia + tipo
- [ ] Implementar `TruncationStrategy` interface con `TruncateMiddle` y `TruncateTail`
- [ ] Implementar pipeline completo: Score → Sort → Select → Truncate
- [ ] Integrar con token counter (issue-06.6) para medición precisa de tokens
- [ ] Integrar con embedding similarity (issue-06.5) para scoring de relevancia
- [ ] Agregar metadata en output: total_tokens, truncated, items_selected, items_omitted
- [ ] Exponer configuración de weights (recent, relevant, structured) vía config del agente

## Tests

- [ ] Test unitario: ContextScorer con datos mock
- [ ] Test unitario: TruncateMiddle preserva head + tail
- [ ] Test unitario: TruncateTail corta desde el final
- [ ] Test unitario: budget exacto no modifica input
- [ ] Test unitario: pool vacío retorna vacío sin error
- [ ] Test unitario: tiebreaker por ID cuando timestamps iguales
- [ ] Test de integración: pipeline completo con token counter real
- [ ] Test E2E: escenarios Gherkin del hu.md
- [ ] Sabotaje: scorer weights en 0 → verificar pipeline falla

## Cierre

- [ ] Verificación manual con datos reales de memoria
- [ ] Suite verde completa
- [ ] Documentar weights recomendados en AGENTS.md o config
