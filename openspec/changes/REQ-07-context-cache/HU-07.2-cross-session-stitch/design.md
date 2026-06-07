# Design: HU-07.2-cross-session-stitch

## Decisión arquitectónica

**Patrón:** Producer/Transformer pipeline.

```
SessionSummaries → Fetcher → DedupTransformer → Formatter → StitchedContext
```

- `SessionStitcher` es el facade
- `SummaryFetcher` recupera summaries desde `MemoryStore` ordenados por recencia descendente
- `DedupEngine` implementa: (1) exact key dedup → (2) semantic dedup (cosine > 0.92) → (3) temporal dedup (misma sesión = colapsar)
- `StitchFormatter` produce texto estructurado con secciones y referencias

## Alternativas descartadas

1. **LLM-based dedup** (preguntar al modelo si dos items son iguales): Caro y lento para el volumen de datos. Preferimos determinístico + embedding.
2. **Stitching completo** (cargar sesiones enteras, no solo summaries): Inviable por token budget. Los summaries son el nivel correcto de granularidad.
3. **Ventana temporal fija** (últimas 24h): Demasiado rígido. Mejor límite configurable por conteo de sesiones.

## Diagrama

```
Nueva Sesión ──▶ SessionStitcher
                      │
                      ▼
              SummaryFetcher ──▶ MemoryStore.GetSessionSummaries(limit=5)
                      │
                      ▼
              DedupEngine ──▶ [exact match] → [semantic match] → [temporal collapse]
                      │
                      ▼
              StitchFormatter ──▶ Decisions[3], OpenItems[2], Skipped[15]
                      │
                      ▼
              StitchedContext ──▶ inyectado en system prompt de la sesión actual
```

## TDD plan

1. **Red:** Test que `Stitch()` con 3 sesiones distintas produce 3 decisiones sin colapsar
2. **Green:** Implementar Stitcher básico con fetcher + exact dedup
3. **Refactor:** Agregar semantic dedup layer
4. **Sabotaje:** Repetir misma decisión en 10 sesiones → verificar 1 salida con 10 referencias

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|-----------|
| Embedding queries N por stitch (caro) | Cachear embedding vectors; limitar a 5 sesiones por defecto |
| Summaries muy grandes | Alimentar output a ContextOptimizer (HU-07.1) para truncamiento |
| Items abiertos que se resuelven en sesión actual | Marcar como "pending" y permitir que la sesión actual los cierre explícitamente |
| Stitching lento en startup de sesión | Ejecutar async mientras se carga UI/CLI; tener timeout de 2s
