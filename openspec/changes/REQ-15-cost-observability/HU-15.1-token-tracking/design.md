# Design: HU-15.1-token-tracking

## Decisión arquitectónica

**Hook pattern en LLM Provider Factory:** Cada provider (OpenAI, Anthropic, Ollama) implementa `Completion()` que devuelve `LLMResponse` con `Usage`. Un middleware hook envuelve el provider y persiste automáticamente.

**Async persistencia:** No bloqueamos la respuesta al usuario. El hook escribe a un channel, un worker persiste en batch cada 5s o 100 registros.

**Tabla token_usage:**
```sql
CREATE TABLE token_usage (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id        UUID NOT NULL,
    run_type      VARCHAR(20) NOT NULL, -- 'domain_agent_run', 'domain_flow_run', 'skill_run'
    step_id       UUID,
    model         VARCHAR(100) NOT NULL,
    provider      VARCHAR(50) NOT NULL,
    input_tokens  INTEGER NOT NULL,
    output_tokens INTEGER NOT NULL,
    total_tokens  INTEGER GENERATED ALWAYS AS (input_tokens + output_tokens) STORED,
    cost          DECIMAL(12,6) NOT NULL DEFAULT 0,
    currency      VARCHAR(3) NOT NULL DEFAULT 'USD',
    cost_unknown  BOOLEAN NOT NULL DEFAULT false,
    metadata      JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_token_usage_run ON token_usage(run_id, run_type);
CREATE INDEX idx_token_usage_created ON token_usage(created_at);
CREATE INDEX idx_token_usage_model ON token_usage(model);
```

**Batch writer:**
```go
type BatchWriter struct {
    buffer    []TokenUsage
    mu        sync.Mutex
    ticker    *time.Ticker
    batchSize int
}

func (bw *BatchWriter) Add(usage TokenUsage) {
    bw.mu.Lock()
    bw.buffer = append(bw.buffer, usage)
    shouldFlush := len(bw.buffer) >= bw.batchSize
    bw.mu.Unlock()
    if shouldFlush {
        bw.Flush()
    }
}

func (bw *BatchWriter) Flush() {
    bw.mu.Lock()
    batch := bw.buffer
    bw.buffer = nil
    bw.mu.Unlock()
    if len(batch) > 0 {
        bw.store.BulkCreate(ctx, batch) // single INSERT with multiple rows
    }
}
```

## Alternativas descartadas

1. **Log-based (structured logs + parsing):** Más frágil, perderíamos estructura de datos y capacidad de query directa.
2. **External service (Datadog, Grafana):** Costo adicional y dependencia externa. Preferimos tener datos en nuestra DB.
3. **Síncrono (persistir antes de responder):** Aumenta latencia de llamadas LLM. Async es mejor.

## Diagrama

```
LLM Call (agent/flow/skill)
  │
  ▼
Provider.Completion()
  │
  ▼
TokenUsageHook.AfterCompletion()
  │
  ▼
BatchWriter.Add(usage)
  │
  ▼ (async, each 5s or 100 records)
BatchWriter.Flush()
  │
  ▼
Store.BulkCreate(usages) → DB token_usage table
```

## TDD plan

1. **Red:** Test hook captura usage después de Completion()
2. **Green:** Implementar hook básico con persistencia síncrona
3. **Refactor:** Extraer BatchWriter, cost calculator
4. **Iterar:** Async batch, aggregations
5. **Sabotaje:** Hook que ignora usage → test detecta

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Buffer nunca se vacía si no hay suficientes registros | Flush periódico cada 5s (ticker) |
| Cost calculator con decimales pierde precisión | Usar DECIMAL(12,6) en DB, float64 con cuidado |
| Run context perdido en llamadas no asociadas | Registrar con run_id = NULL, run_type = 'standalone' |
