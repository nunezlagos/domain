# Proposal: issue-06.6-token-count-stream

## Intención

Implementar una librería de streaming con conciencia de tokens. Permite trackear el consumo de tokens en tiempo real durante respuestas streaming de LLMs, cortar la generación cuando se excede un presupuesto, y dividir respuestas largas en chunks con tamaño basado en tokens (no caracteres).

## Scope

**Incluye:**
- `StreamTracker`: envuelve un `<-chan StreamChunk` y añade cumulative_tokens a cada chunk
- `TokenBudget`: configuración de max_tokens y/o max_seconds. Compartible entre llamadas.
- `TokenCountingProvider`: decorator de Provider que añade tracking automático
- `Chunker`: divide respuesta larga en chunks de N tokens
- `ChunkMetadata`: chunk_index, is_last, cumulative_tokens, finish_reason
- Interrupción automática del stream cuando se excede el budget

**Excluye:**
- Cost tracking (issue-06.4, issue-15.1)
- WebSocket delivery (solo la capa de chunking)
- Reintentos

## Enfoque técnico

- `StreamTracker` recibe un `<-chan StreamChunk` y devuelve un `<-chan TrackedChunk` que añade `cumulative_tokens`.
- `TokenBudget` usa un contador atómico compartido. `Allow(count) bool` verifica si hay presupuesto.
- `TokenCountingProvider` implementa `Provider` delegando en otro provider, pero trackeando tokens en streaming.
- `Chunker` acumula tokens hasta llegar a chunk_size, luego emite el chunk. Usa `tokenCounter.Count()` para medir.
- El chunking se hace en el server, antes de enviar al cliente.

## Riesgos

- **Performance de conteo:** Contar tokens en cada chunk puede ser lento si el chunk es pequeño. Mitigación: chunk_size mínimo de 50 tokens.
- **Budget compartido:** Race condition en el contador atómico. Mitigación: atomic.AddInt64.
- **Chunking exacto:** Saber exactamente dónde cortar para que el chunk tenga exactamente N tokens es difícil. Mitigación: chunk_size como target, no exacto. Cortar en el límite de palabra más cercano.

## Testing

- **Unitarios:** StreamTracker añade cumulative_tokens correctamente, TokenBudget allow/deny, Chunker divide en chunks.
- **Integración:** TokenCountingProvider wrapping un provider mock streaming, verificar corte por budget.
- **Sabotaje:** Budget excedido exactamente en el límite.
