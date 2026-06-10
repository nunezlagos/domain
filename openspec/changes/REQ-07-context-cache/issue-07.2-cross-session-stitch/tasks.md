# Tasks: issue-07.2-cross-session-stitch

## Backend

- [ ] Implementar `SessionStitcher` facade con método `Stitch(ctx, sessionID)`
- [ ] Implementar `SummaryFetcher` que recupera summaries desde `MemoryStore` ordenados por recencia
- [ ] Implementar `DedupEngine` con exact key dedup + semantic dedup (cosine > 0.92)
- [ ] Implementar `StitchFormatter` que produce output con secciones Decisions, OpenItems, RecurringContext
- [ ] Agregar límite configurable de sesiones máximas a stitch (default: 5)
- [ ] Indicar en output cuántas sesiones se omitieron por límite
- [ ] Integrar con sistema de memoria (REQ-03) para GetSessionSummaries
- [ ] Integrar con embedding similarity (issue-06.5) para dedup semántico

## Tests

- [ ] Test unitario: Stitcher con summaries mock, 3 sesiones distintas
- [ ] Test unitario: exact dedup (misma semantic key)
- [ ] Test unitario: semantic dedup (cosine > threshold)
- [ ] Test unitario: límite de sesiones respetado
- [ ] Test unitario: sin sesiones previas retorna empty
- [ ] Test de integración: Stitcher + MemoryStore real
- [ ] Test E2E: escenarios Gherkin del hu.md
- [ ] Sabotaje: misma decisión en 10 sesiones → 1 salida con 10 referencias

## Cierre

- [ ] Verificación manual con historial de sesiones real
- [ ] Suite verde completa
- [ ] Documentar límite de sesiones y threshold de dedup en configuración
