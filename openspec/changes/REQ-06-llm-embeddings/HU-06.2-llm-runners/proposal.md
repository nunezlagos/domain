# Proposal: HU-06.2-llm-runners

## Intención

Implementar tres runners LLM concretos que implementen la interfaz `Provider` definida en HU-06.1. Cada runner traduce la interfaz genérica al API específica del proveedor (OpenAI, Anthropic, Google), manejando autenticación, retry con backoff, rate limiting, y streaming.

## Scope

**Incluye:**
- `OpenAIRunner`: gpt-4o, gpt-4o-mini. API v1/chat/completions. OpenAI Go SDK.
- `AnthropicRunner`: claude-sonnet-4, claude-haiku. API v1/messages. Anthropic Go SDK.
- `GoogleRunner`: gemini-2.0-flash. API v1beta/models. Google AI Go SDK.
- Retry policy: 3 retries con exponential backoff (100ms, 500ms, 2s) para 429/5xx.
- Rate limiting: semáforo por provider (max 10 concurrent requests configurable).
- Auth: API key desde env vars.
- Streaming: traducción de server-sent events a channel de StreamChunk.
- Tests con HTTP mocks (gohttptest) para evitar llamadas reales.

**Excluye:**
- Ollama (HU-06.3, separado por ser local)
- Embeddings (HU-06.5)
- Token counting avanzado (HU-06.6, aunque usamos Usage del API)

## Enfoque técnico

- Cada runner es un struct que implementa `Provider`. Recibe config (API key, base URL opcional, timeout) en el constructor.
- `Complete` traduce `CompletionOpts` al body específico del API, llama al endpoint, parsea response.
- `CompleteStream` usa SSE (Server-Sent Events) para OpenAI/Google, streaming nativo para Anthropic.
- Retry con `cenkalti/backoff` o implementación propia con jitter.
- HTTP client con timeout default 60s.
- Tests con `net/http/httptest.Server` que simula respuestas del API.

## Riesgos

- **Breaking changes en APIs externas:** Los SDKs oficiales mitigan esto parcialmente. Usar versiones pinneadas.
- **Costos:** Tests contra APIs reales generan costos. Tests unitarios con mocks, integración opcional contra APIs reales.
- **Rate limiting agresivo:** Cada provider tiene límites distintos. Implementar semáforo configurable.

## Testing

- **Unitarios:** Cada runner con HTTP mock, verificar serialización/deserialización correcta.
- **Integración (opcional):** Con API keys reales en CI/CD protegido.
- **Retry:** Mock HTTP que retorna 429 dos veces y 200 a la tercera.
- **Streaming:** Mock SSE endpoint, verificar chunks.
