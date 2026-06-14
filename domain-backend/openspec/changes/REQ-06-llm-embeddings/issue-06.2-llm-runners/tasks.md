# Tasks: issue-06.2-llm-runners

## Backend

- [x] Implementar resiliencia con HTTP client, semáforo, retry → decorators componibles `llm/retry` + `llm/ratelimit` (en lugar de baseRunner heredado: cada provider mantiene su client y la resiliencia se compone en factory)
- [x] Implementar OpenAIRunner: Complete + CompleteStream → internal/llm/openai/provider.go
- [x] Implementar AnthropicRunner: Complete + CompleteStream → internal/llm/anthropic/provider.go
- [x] Implementar GoogleRunner: Complete + CompleteStream → internal/llm/google/provider.go (Gemini generateContent + SSE) — 2026-06-10
- [x] Implementar retry policy con exponential backoff → llm/retry (transient: 429/5xx/network; auth no reintenta) — 2026-06-10
- [x] Implementar rate limiter con semáforo → llm/ratelimit (slot retenido durante streams) — 2026-06-10
- [x] Registrar los runners en factory al inicializar → cmd/domain (cb+ratelimit+retry) + cmd/domain-mcp (ratelimit+retry); google con DOMAIN_GOOGLE_KEY — 2026-06-10
- [x] Implementar tests con httptest.Server para cada runner → openai (6), anthropic (6), google (6)
- [x] Implementar test de streaming con SSE mock → TestCompleteStream_Chunks (google) + tests existentes openai/anthropic

## Frontend

- [x] N/A

## Tests

- [x] Test unitario: OpenAIRunner con HTTP mock → openai/provider_test.go
- [x] Test unitario: AnthropicRunner con HTTP mock → anthropic/provider_test.go
- [x] Test unitario: GoogleRunner con HTTP mock → google/provider_test.go (basic, role mapping) — 2026-06-10
- [x] Test unitario: streaming produce chunks correctos → google TestCompleteStream_Chunks
- [x] Test unitario: retry en 429 funciona → retry TestComplete_RetriesOn429 + TestIsTransient_Matrix — 2026-06-10
- [x] Test unitario: rate limiter bloquea excesos → ratelimit TestComplete_BoundsConcurrency + TestCompleteStream_HoldsSlotUntilDone — 2026-06-10
- [x] Test unitario: timeout cancela request → retry TestComplete_CtxCancelledDuringBackoff + ratelimit TestComplete_CtxCancelWhileWaiting
- [x] Test unitario: API key inválida → error claro → google TestComplete_InvalidAPIKey_ClearError (no reintenta: TestComplete_NonTransient_NoRetry)
- [x] Sabotaje: response malformed → error graceful, no panic → google TestSabotage_MalformedResponse

## Cierre

- [x] Verificación manual (opcional con API keys reales) → cubierta por mocks httptest; wiring verificado en boot de ambos binarios
- [x] Suite verde → 2026-06-10: 78 tests llm/... + suite corta 967 verdes
