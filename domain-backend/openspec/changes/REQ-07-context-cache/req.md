# REQ-07-context-cache: Gestión de contexto: optimización de ventana, truncamiento inteligente, stitching cross-session, caché semántico de LLM, token budget.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F3

## Descripción

Gestión de contexto: optimización de ventana, truncamiento inteligente, stitching cross-session, caché semántico de LLM, token budget.

## Criterios de éxito

- Context optimizer selecciona fragmentos priorizando recent > relevant > structured con truncamiento inteligente respetando ventana del modelo target
- Cross-session stitching fusiona resúmenes de sesiones anteriores con dedup semántico configurable
- Caché semántico LLM con lookup por similitud de embeddings, TTL configurable y métrica de hit-rate
- Token budget manager con hard/soft limits, tracking en streaming y validación contra model registry de REQ-06

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-07.1-context-optimizer | propuesta | Optimizador de ventana de contexto: selecciona fragmentos más relevantes por prioridad recent > relevant > structured con truncamiento inteligente |
| issue-07.2-cross-session-stitch | propuesta | Stitching cross-session: fusiona resúmenes de sesiones anteriores con dedup semántico |
| issue-07.3-llm-semantic-cache | propuesta | Caché semántico de LLM: respuestas cacheadas por similitud de embeddings con TTL configurable |
| issue-07.4-token-budget | propuesta | Token budget manager: hard/soft limits, tracking streaming, validación vs model registry |
