# REQ-06-llm-embeddings: Abstracción de providers LLM y embeddings: OpenAI, Anthropic, Google, Ollama. Model registry, cost tracking, pgvector, streaming.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F2

## Descripción

Abstracción de providers LLM y embeddings: OpenAI, Anthropic, Google, Ollama. Model registry, cost tracking, pgvector, streaming.

## Criterios de éxito

- Factory de providers LLM con interfaz común
- Runners funcionales para OpenAI, Anthropic, Google y Ollama
- Model registry con precios y cálculo de costos
- Embedding providers integrados con pgvector
- Streaming con token budget tracking y chunked output

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-06.1-llm-provider-factory | proposed | Factory pattern: Provider interface, registry thread-safe, config via env vars |
| HU-06.2-llm-runners | proposed | OpenAI (gpt-4o/mini), Anthropic (claude-sonnet-4/haiku), Google (gemini-2.0-flash) runners |
| HU-06.3-ollama-runner | proposed | Ollama runner para LLMs locales, config via DOMAIN_OLLAMA_URL |
| HU-06.4-model-registry-cost | proposed | Central model registry: precios por token, CalculateCost, token counters |
| HU-06.5-embedding-pgvector | proposed | Embedding providers (OpenAI, Anthropic), pgvector integration, índices IVFFlat/HNSW |
| HU-06.6-token-count-stream | proposed | Streaming con token tracking, TokenBudget, chunked output, decorator pattern |
