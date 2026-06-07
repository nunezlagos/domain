# Design: HU-06.2-llm-runners

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativa | Razón |
|----------|---------------|-------------|-------|
| HTTP client | net/http + httptest para tests | SDK oficiales | Control total, sin dependencias pesadas |
| Retry | Cenkalti/backoff v4 | Manual | Probado, configurable, jitter |
| Rate limiter | Semáforo (chan struct{}) | Token bucket | Simple, suficiente para control concurrente |
| Streaming | Channel + goroutine | Callback pattern | Mejor integración con select/context |
| Auth | API key en header | OAuth | Simple, soportado por todos los providers |

## Alternativas descartadas

- **SDKs oficiales:** Agregan dependencias, versiones, y a menudo wrappers delgados sobre HTTP. Preferimos HTTP directo para control y testing.
- **Callback pattern:** Menos composable que channels. Channels permiten timeout, cancelación y merge.
- **Token bucket:** Más complejo de lo necesario. El semáforo simple controla concurrencia máxima.

## Diagrama

```
internal/llm/providers/
├── openai.go
│   └── OpenAIRunner
│       ├── Complete: POST https://api.openai.com/v1/chat/completions
│       └── CompleteStream: POST con stream=true → SSE parsing
├── anthropic.go
│   └── AnthropicRunner
│       ├── Complete: POST https://api.anthropic.com/v1/messages
│       └── CompleteStream: POST con stream=true → evento chunk
└── google.go
    └── GoogleRunner
        ├── Complete: POST https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent
        └── CompleteStream: POST con stream=true → SSE
```

### Estructura común

```go
type baseRunner struct {
    apiKey    string
    baseURL   string
    httpClient *http.Client
    sem       chan struct{}  // rate limiter
}

func (r *baseRunner) doRequest(ctx context.Context, method, path string, body, resp any) error {
    // Build request, set headers, do with retry
}

// Cada runner incrusta baseRunner y agrega su lógica específica
type OpenAIRunner struct {
    baseRunner
    orgID string
}
```

### Retry policy

```go
func withRetry(operation func() error) error {
    b := backoff.NewExponentialBackOff()
    b.InitialInterval = 100 * time.Millisecond
    b.MaxInterval = 5 * time.Second
    b.MaxElapsedTime = 30 * time.Second
    return backoff.Retry(operation, backoff.WithContext(b, ctx))
}
```

## TDD plan

1. **TestOpenAIComplete:** Mock HTTP → response parseado correctamente
2. **TestOpenAIStream:** Mock SSE → chunks correctos
3. **TestOpenAIModels:** Lista modelos soportados
4. **TestAnthropicComplete:** Mock HTTP → response parseado
5. **TestAnthropicStream:** Mock streaming → chunks
6. **TestGoogleComplete:** Mock HTTP → response parseado
7. **TestGoogleStream:** Mock SSE → chunks
8. **TestRetry429:** Mock que falla 2 veces → 3ra exitosa
9. **TestRetryMax:** Mock que falla 4 veces → error final
10. **TestAuthError:** Mock 401 → error autenticación
11. **TestTimeout:** Context cancelado → error
12. **TestRateLimiter:** 20 requests concurrentes → solo 10 pasan el semáforo
13. **TestSabotaje:** Provider retorna JSON inválido → error graceful

## Riesgos y mitigación

- **API keys en tests:** Usar httptest.Server para no hacer llamadas reales. Las integraciones reales son opt-in.
- **Campos de response cambiantes:** Parseo flexible con raw JSON + solo los campos que necesitamos.
- **Streaming incompleto:** Timeout por chunk (30s entre chunks). Si expira, error.
