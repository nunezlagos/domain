# Proposal: HU-26.5-circuit-breaker-llm

## Intención

Circuit breaker por (provider, model) en LLM client abstraction (REQ-06) con fallback declarativo per-agent y half-open recovery.

## Scope

- gobreaker per (provider, model) tuple
- Agents declaran `fallback_models: []` opcional
- Embedding fallback queue (async retry)
- Métricas + alertas
- Tests con LLM provider mock que simula outage

## Riesgos

- Fallback quality drop: warning explícito en logs + métrica
- Memory N CBs: cap top 100 most-used; resto sin CB (fail-fast normal)

## Testing

- 5 errores → OPEN
- Fallback model usado
- Half-open recovery
- Per-provider isolation
- Embedding queue retry
