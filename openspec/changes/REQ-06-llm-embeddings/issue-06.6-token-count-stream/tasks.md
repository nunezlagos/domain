# Tasks: issue-06.6-token-count-stream

## Backend

- [x] Implementar StreamTracker con cumulative tokens
- [x] Implementar TokenBudget con maxTokens y maxSeconds (thread-safe)
- [x] Implementar TokenCountingProvider (decorator)
- [x] Implementar Chunker para dividir respuestas en chunks de N tokens
- [x] Implementar corte automático cuando se excede el budget (finish_reason="token_limit")
- [x] Integrar StreamTracker con los runners existentes
- [x] Implementar tests con streams mock

## Frontend

- [x] N/A (la capa de chunking es server-side)

## Tests

- [x] Test unitario: StreamTracker acumula tokens correctamente
- [x] Test unitario: TokenBudget allow/exceed
- [x] Test unitario: TokenBudget timeout
- [x] Test unitario: TokenBudget compartido entre streams
- [x] Test unitario: Chunker produce chunks de tamaño correcto
- [x] Test unitario: Chunker metadatos (índice, último)
- [x] Test integración: TokenCountingProvider con provider mock
- [x] Test integración: corte por budget en streaming
- [x] Sabotaje: budget exacto → corte preciso

## Cierre

- [x] Verificar manual con streaming real
- [x] Suite verde
