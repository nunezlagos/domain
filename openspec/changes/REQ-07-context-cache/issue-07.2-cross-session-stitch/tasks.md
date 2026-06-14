# Tasks: issue-07.2-cross-session-stitch

## Backend

- [x] Implementar `SessionStitcher` facade con método `Stitch(ctx, sessionID)`
- [x] Implementar `SummaryFetcher` que recupera summaries desde `MemoryStore` ordenados por recencia
- [x] Implementar `DedupEngine` con exact key dedup + semantic dedup (cosine > 0.92)
- [x] Implementar `StitchFormatter` que produce output con secciones Decisions, OpenItems, RecurringContext
- [x] Agregar límite configurable de sesiones máximas a stitch (default: 5)
- [x] Indicar en output cuántas sesiones se omitieron por límite
- [x] Integrar con sistema de memoria (REQ-03) para GetSessionSummaries
- [x] Integrar con embedding similarity (issue-06.5) para dedup semántico

## Tests

- [x] Test unitario: Stitcher con summaries mock, 3 sesiones distintas
- [x] Test unitario: exact dedup (misma semantic key)
- [x] Test unitario: semantic dedup (cosine > threshold)
- [x] Test unitario: límite de sesiones respetado
- [x] Test unitario: sin sesiones previas retorna empty
- [x] Test de integración: Stitcher + MemoryStore real
- [x] Test E2E: escenarios Gherkin del hu.md
- [x] Sabotaje: misma decisión en 10 sesiones → 1 salida con 10 referencias

## Cierre

- [x] Verificación manual con historial de sesiones real
- [x] Suite verde completa
- [x] Documentar límite de sesiones y threshold de dedup en configuración
