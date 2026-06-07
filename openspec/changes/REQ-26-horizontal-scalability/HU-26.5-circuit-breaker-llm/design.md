# Design: HU-26.5-circuit-breaker-llm

## Wrapper LLM client

```go
type CircuitedLLMClient struct {
  inner   LLMClient
  breakers sync.Map  // key: provider+":"+model → *gobreaker.CircuitBreaker
}

func (c *CircuitedLLMClient) Chat(ctx, req) (*Response, error) {
  cb := c.breakerFor(req.Provider, req.Model)
  result, err := cb.Execute(func() (any, error) {
    return c.inner.Chat(ctx, req)
  })
  if err != nil && errors.Is(err, gobreaker.ErrOpenState) {
    return nil, ErrLLMProviderUnavailable{Provider: req.Provider, Model: req.Model}
  }
  return result.(*Response), err
}
```

## Agent config

```sql
ALTER TABLE agents
  ADD COLUMN fallback_models TEXT[] DEFAULT '{}';
-- formato: "anthropic/claude-sonnet-4-6", "ollama/llama3"
```

## Embedding queue fallback

```sql
CREATE TABLE embedding_queue (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  entity_type VARCHAR(50),
  entity_id UUID,
  content TEXT,
  attempts INT DEFAULT 0,
  next_attempt_at TIMESTAMPTZ,
  status VARCHAR(20),  -- pending|done|dead
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

Worker consume queue cuando CB de embeddings está CLOSED.

## TDD plan

1. 5 errores → CB OPEN
2. Fallback model usado cuando OPEN
3. Half-open probe
4. Per-provider isolation
5. Embedding queue retry
6. Métricas observables
