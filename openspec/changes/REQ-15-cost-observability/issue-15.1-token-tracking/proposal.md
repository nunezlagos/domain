# Proposal: issue-15.1-token-tracking

## Intención

Implementar un sistema de tracking automático que registre cada llamada LLM con sus métricas de tokens y costo calculado. Los registros se asocian a runs (agent, flow, skill) y permiten agregaciones por proyecto, modelo, proveedor y fecha.

## Scope

**Incluye:**
- Tabla `token_usage` con campos: id, run_id, run_type, step_id, model, provider, input_tokens, output_tokens, total_tokens, cost, cost_currency, cost_unknown, metadata, created_at
- Hook en LLM Provider Factory que persiste después de cada Completion()
- Cost calculator que usa model registry para precios por token
- Consultas de agregación por: domain_agent_run, domain_flow_run, project, model, provider, date_range
- API endpoints (via CRUD factory de REQ-13)
- TTL policy para datos viejos (configurable)

**Excluye:**
- Dashboards (issue-15.2)
- Alertas (issue-15.3)
- Export CSV (issue-15.2)

## Enfoque técnico

**Hook en LLM Provider:**
```go
type TokenUsageHook struct {
    store TokenUsageStore
}

func (h *TokenUsageHook) AfterCompletion(ctx context.Context, req *LLMRequest, resp *LLMResponse, runInfo RunInfo) {
    usage := TokenUsage{
        RunID:        runInfo.RunID,
        RunType:      runInfo.RunType, // "domain_agent_run", "domain_flow_run", "skill_run"
        StepID:       runInfo.StepID,
        Model:        resp.Model,
        Provider:     resp.Provider,
        InputTokens:  resp.Usage.InputTokens,
        OutputTokens: resp.Usage.OutputTokens,
        TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
        Cost:         calculateCost(resp.Model, resp.Usage),
        Timestamp:    time.Now(),
    }
    h.store.Create(ctx, &usage)
}
```

**Cost calculator:**
```go
func calculateCost(model string, usage *Usage) float64 {
    prices, ok := modelRegistry.GetPricing(model)
    if !ok {
        return 0 // model not found, cost unknown
    }
    inputCost := (float64(usage.InputTokens) / 1000) * prices.InputPricePer1K
    outputCost := (float64(usage.OutputTokens) / 1000) * prices.OutputPricePer1K
    return inputCost + outputCost
}
```

**Aggregation queries:**
```go
type TokenAggregation struct {
    TotalTokens  int64   `json:"total_tokens"`
    TotalCost    float64 `json:"total_cost"`
    InputTokens  int64   `json:"input_tokens"`
    OutputTokens int64   `json:"output_tokens"`
    Count        int64   `json:"count"`
}

func (s *store) AggregateByProject(ctx, projectID, from, to) (*TokenAggregation, error)
func (s *store) AggregateByModel(ctx, from, to) (map[string]*TokenAggregation, error)
func (s *store) AggregateByRun(ctx, runID) (*TokenAggregation, error)
```

## Riesgos

| Riesgo | Mitigación |
|--------|------------|
| DB crece rápido (muchas llamadas LLM) | TTL policy + particionado por mes, archivo automático |
| Cost calculator desactualizado si cambian precios | Model registry editable via API, precios actualizables |
| Hook impacta performance de llamadas LLM | Async persist (goroutine + buffer), no bloquear respuesta |
| Run context no disponible en algunas llamadas | RunInfo opcional, registrar sin asociación si no hay contexto |

## Testing

- Unit: cost calculator con precios mock
- Integration: hook persiste en DB
- Integration: aggregations queries con datos mock
- Sabotaje: hook que no persiste → test detecta
