# Tasks: issue-06.2-llm-runners

## Backend

- [ ] Implementar baseRunner con HTTP client, semáforo, retry
- [ ] Implementar OpenAIRunner: Complete + CompleteStream
- [ ] Implementar AnthropicRunner: Complete + CompleteStream
- [ ] Implementar GoogleRunner: Complete + CompleteStream
- [ ] Implementar retry policy con exponential backoff
- [ ] Implementar rate limiter con semáforo
- [ ] Registrar los 3 runners en factory al inicializar
- [ ] Implementar tests con httptest.Server para cada runner
- [ ] Implementar test de streaming con SSE mock

## Frontend

- [ ] N/A

## Tests

- [ ] Test unitario: OpenAIRunner con HTTP mock
- [ ] Test unitario: AnthropicRunner con HTTP mock
- [ ] Test unitario: GoogleRunner con HTTP mock
- [ ] Test unitario: streaming produce chunks correctos
- [ ] Test unitario: retry en 429 funciona
- [ ] Test unitario: rate limiter bloquea excesos
- [ ] Test unitario: timeout cancela request
- [ ] Test unitario: API key inválida → error claro
- [ ] Sabotaje: response malformed → error graceful, no panic

## Cierre

- [ ] Verificación manual (opcional con API keys reales)
- [ ] Suite verde
