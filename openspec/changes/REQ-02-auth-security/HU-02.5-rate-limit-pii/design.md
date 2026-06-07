# Design: HU-02.5-rate-limit-pii

## Decisión arquitectónica

### Rate Limiting
**Algoritmo:** Token bucket en memoria por API key.
**Estructura:** `sync.Map` con `*Bucket` values, cleanup periódico.
**Middleware:** Se ejecuta antes de auth middleware para bloquear temprano.

### PII Redaction
**Enfoque:** Regex `<private>.*?</private>` → reemplazar con `[REDACTED]`.
**Ubicación:** `internal/sanitize/pii.go` — función pública sin estado.
**Aplicación:** Storage layer (al escribir), logging hook (al loguear).

## Alternativas descartadas

- **Redis para rate limit:** Sobredimensionado para single-instance MVP. Agregar cuando haya múltiples réplicas.
- **Sliding window log:** Más preciso pero más costoso en memoria. Token bucket es suficiente.
- **PII con AST parser:** Sobredimensionado. Regex con tags explícitas es suficiente.
- **Eliminar PII en vez de redactar:** `[REDACTED]` deja claro que había información sensible.

## Diagrama

```
Rate Limiting:

Request → RateLimitMiddleware
  │
  └─→ Extract API key (de header o contexto)
  └─→ bucket = buckets.LoadOrStore(key, newBucket(limit, window))
  └─→ bucket.Allow()
        ├─→ tokens > 0 → consume 1 token → next handler
        └─→ tokens <= 0 → 429 + Retry-After + headers

Cleanup goroutine (cada 10 min):
  └─→ Range over buckets
        └─→ if lastAccess > 2*window → Delete

PII Redaction:

Input: "Hola <private>secreto</private> mundo"
  → Sanitize()
  → "Hola [REDACTED] mundo"

Aplicación:
  Storage layer:
    observation.Content = sanitize.Sanitize(observation.Content)
    prompt.Content = sanitize.Sanitize(prompt.Content)

  Logging hook (Logrus):
    entry.Message = sanitize.Sanitize(entry.Message)
    for k, v := range entry.Data {
      if str, ok := v.(string); ok {
        entry.Data[k] = sanitize.Sanitize(str)
      }
    }
```

## Rate Limiter

```go
type TokenBucket struct {
    mu       sync.Mutex
    tokens   float64
    lastRefill time.Time
}

type RateLimiter struct {
    buckets   sync.Map
    limit     float64   // requests
    window    time.Duration
    rate      float64   // tokens per second = limit / window.Seconds()
}

func (rl *RateLimiter) Allow(key string) (bool, time.Duration) {
    bucket, _ := rl.buckets.LoadOrStore(key, &TokenBucket{
        tokens: rl.limit,
        lastRefill: time.Now(),
    })
    bucket.mu.Lock()
    defer bucket.mu.Unlock()
    bucket.refill(rl.rate)
    if bucket.tokens >= 1 {
        bucket.tokens--
        return true, 0
    }
    wait := time.Duration((1 - bucket.tokens) / rl.rate * float64(time.Second))
    return false, wait
}
```

## PII Sanitizer

```go
package sanitize

import "regexp"

var piiPattern = regexp.MustCompile(`<private>.*?</private>`)
var redacted = []byte("[REDACTED]")

func Sanitize(s string) string {
    return string(piiPattern.ReplaceAll([]byte(s), redacted))
}

func SanitizeJSON(data []byte) []byte {
    return piiPattern.ReplaceAll(data, redacted)
}
```

## Config

```go
type RateLimitConfig struct {
    Requests int           // default: 100
    Window   time.Duration // default: 1 minute
}
```

## TDD plan

### Rate Limit
1. Test Allow() consume tokens hasta límite
3. Test Allow() retorna false después de límite
4. Test refill después de esperar window/límite
5. Test headers de respuesta: X-RateLimit-Remaining, X-RateLimit-Reset, Retry-After
6. Test middleware retorna 429 cuando corresponde
7. Test diferentes API keys tienen buckets independientes
8. Test cleanup de keys inactivas

### PII
9. Test Sanitize reemplaza contenido entre `<private>` tags
10. Test Sanitize no modifica texto sin tags
11. Test Sanitize con múltiples tags
12. Test Sanitize con tags vacías `<private></private>`
13. Test SanitizeJSON funciona con JSON strings
14. Test storage llama a Sanitize antes de escribir
15. Test log hook sanitiza antes de loguear

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Memory leak rate limit buckets | Media | Bajo | Cleanup goroutine cada 10 min |
| Race condition en Allow() | Baja | Medio | sync.Mutex por bucket |
| Regex catastrófico con input malicioso | Baja | Medio | Regex non-greedy `.*?` + test con inputs largos |
| Olvidar sanitizar en algún storage path | Media | Alto | Code review; testing de integración |
| Falso positivo: texto contiene `<private>` literal | Baja | Bajo | Documentar convención de tags |
