# Tasks: issue-06.6-token-count-stream

## Backend

- [ ] Implementar StreamTracker con cumulative tokens
- [ ] Implementar TokenBudget con maxTokens y maxSeconds (thread-safe)
- [ ] Implementar TokenCountingProvider (decorator)
- [ ] Implementar Chunker para dividir respuestas en chunks de N tokens
- [ ] Implementar corte automático cuando se excede el budget (finish_reason="token_limit")
- [ ] Integrar StreamTracker con los runners existentes
- [ ] Implementar tests con streams mock

## Frontend

- [ ] N/A (la capa de chunking es server-side)

## Tests

- [ ] Test unitario: StreamTracker acumula tokens correctamente
- [ ] Test unitario: TokenBudget allow/exceed
- [ ] Test unitario: TokenBudget timeout
- [ ] Test unitario: TokenBudget compartido entre streams
- [ ] Test unitario: Chunker produce chunks de tamaño correcto
- [ ] Test unitario: Chunker metadatos (índice, último)
- [ ] Test integración: TokenCountingProvider con provider mock
- [ ] Test integración: corte por budget en streaming
- [ ] Sabotaje: budget exacto → corte preciso

## Cierre

- [ ] Verificar manual con streaming real
- [ ] Suite verde
