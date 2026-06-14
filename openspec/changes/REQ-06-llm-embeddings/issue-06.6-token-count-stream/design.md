# Design: issue-06.6-token-count-stream

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativa | Razón |
|----------|---------------|-------------|-------|
| Stream tracking | Decorator pattern | Embed en provider | Separa concerns, reusable |
| Budget contador | sync/atomic | mutex | Performance, sin locks |
| Chunk size | Target tokens (no exacto) | Exacto | Costo de contar exactamente es alto |
| Chunk boundary | Word boundary | Character boundary | Lectura natural |

## Alternativas descartadas

- **Chunking exacto:** Requeriría contar tokens de cada posible punto de corte, O(n²). Target + word boundary es más eficiente.
- **Embed en provider:** Mezcla responsabilidades. Decorator es más limpio y testeable.

## Diagrama

```
Provider.CompleteStream → <-chan StreamChunk
        │
        ▼
StreamTracker.Wrap(ctx, provider.CompleteStream, budget)
        │
        ├─► Por cada chunk: cumulative_tokens += count(chunk.Content)
        ├─► Si budget.Allow(cumulative) → false → cerrar stream con "token_limit"
        └─► Emitir TrackedChunk { Content, Done, CumulativeTokens, FinishReason }

        │
        ▼
Chunker.Chunk(stream, chunkSize)
        │
        └─► Acumular tokens hasta chunkSize → emitir chunk completed
        └─► Cada chunk: { Content, ChunkIndex, IsLast, CumulativeTokens }
```

### Structs

```go
type TrackedChunk struct {
    Content          string
    Done             bool
    CumulativeTokens int
    FinishReason     string // "stop", "length", "token_limit", "timeout"
    TotalTokens      int    // solo en Done=true
}

type TokenBudget struct {
    maxTokens    int64
    maxSeconds   time.Duration
    used         atomic.Int64
    startTime    time.Time
}

func (b *TokenBudget) Allow(tokens int64) bool {
    if b.maxTokens > 0 && b.used.Load()+tokens > b.maxTokens {
        return false
    }
    if b.maxSeconds > 0 && time.Since(b.startTime) > b.maxSeconds {
        return false
    }
    return true
}

func (b *TokenBudget) Add(tokens int64) {
    b.used.Add(tokens)
}
```

### TokenCountingProvider

```go
type TokenCountingProvider struct {
    inner  Provider
    budget *TokenBudget
    counter TokenCounter
}

func (p *TokenCountingProvider) CompleteStream(ctx context.Context, prompt string, opts CompletionOpts) (<-chan StreamChunk, error) {
    innerCh, err := p.inner.CompleteStream(ctx, prompt, opts)
    if err != nil {
        return nil, err
    }
    return p.trackStream(ctx, innerCh), nil
}
```

## TDD plan

1. **TestStreamTracker:** Chunks → cumulative_tokens incrementa
2. **TestStreamTrackerDone:** Último chunk tiene Done=true y TotalTokens
3. **TestTokenBudgetAllow:** Dentro del budget → true
4. **TestTokenBudgetExceeded:** Excede maxTokens → false
5. **TestTokenBudgetTimeout:** Excede maxSeconds → false
6. **TestTokenBudgetCompartido:** 2 streams mismo budget → segundo se rechaza si excede
7. **TestChunker:** Stream largo → chunks de ~N tokens
8. **TestChunkerMetadatos:** Cada chunk tiene chunk_index, is_last
9. **TestTokenCountingProvider:** Wrap de provider → chunks tienen cumulative tokens
10. **TestTokenCountingProviderCutoff:** Budget excedido → stream se cierra con token_limit
11. **TestSabotaje:** Budget exactamente al límite → corte preciso

## Riesgos y mitigación

- **Conteo en cada chunk:** Usar tiktoken para OpenAI, cachear encoding. Chunk size mínimo 50 tokens.
- **Budget compartido entre requests:** El budget es por contexto (request/agente), no global.
- **Chunking impreciso:** El target es aproximado. +/- 10% es aceptable.
