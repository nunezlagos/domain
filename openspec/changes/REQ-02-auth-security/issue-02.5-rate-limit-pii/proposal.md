# Proposal: issue-02.5-rate-limit-pii

## Intención

Proteger la plataforma con dos mecanismos de defensa complementarios: (1) rate limiting por API key usando token bucket para prevenir abuso, y (2) redacción automática de datos PII marcados con tags `<private>` para evitar exposición de información sensible en storage y logs.

## Scope

**Incluye:**
- Token bucket rate limiter en memoria (por API key)
- Configurable: requests por ventana de tiempo
- Headers de rate limit estándar: X-RateLimit-Remaining, X-RateLimit-Reset, Retry-After
- Middleware de rate limit que se aplica antes de autenticación (defense in depth)
- Rate limits globales por defecto (100/min)
- PII redaction: función `Sanitize(text string) string` que reemplaza `<private>...</private>` por `[REDACTED]`
- Middleware PII que sanitiza request bodies antes de logging
- Integración PII en storage layer (al escribir observaciones, prompts, etc.)
- Configuración via env vars: `DOMAIN_RATE_LIMIT_REQUESTS`, `DOMAIN_RATE_LIMIT_WINDOW`

**No incluye:**
- Rate limiting distribuido (Redis) — solo en memoria (single instance)
- Rate limiting por IP
- PII detection automática (solo por tags explícitas)
- Diferentes rate limits por endpoint

## Enfoque técnico

### Rate Limiting
1. Token bucket algorithm: cada API key tiene un bucket con N tokens
2. Se rellena a razón de N/window tokens por segundo
3. Implementación en memoria con `sync.Map` + goroutine de cleanup periódico
4. Bucket struct: tokens (float64), lastRefill (time.Time)
5. Middleware extrae API key del header, aplica Allow(key) → 429 si no hay tokens
6. Headers de respuesta estándar

### PII Redaction
1. `internal/sanitize/pii.go` con regex `<private>.*?</private>` (non-greedy)
2. `Sanitize()` aplicable a strings, `SanitizeJSON()` para JSON marshaled
3. Middleware que sanitiza request body antes de logging
4. Storage layer llama a `Sanitize()` antes de INSERT/UPDATE
5. Logrus/zerolog hook que sanitiza campos de log

## Riesgos

- **Rate limit en memoria se pierde al reiniciar:** Aceptable. Los buckets se reinician.
- **Memory leak por keys inactivas:** Mitigación: cleanup goroutine cada 10 minutos remueve buckets no usados > 2 ventanas.
- **Race conditions en token bucket:** Mitigación: sync.Mutex por bucket o atomic operations.
- **PII regex demasiado amplio:** `<private>` tags explícitas minimizan falsos positivos.
- **Performance de regex en cada request:** Benchmark: regex compilado una vez, ejecución es O(n).

## Testing

- Test token bucket: consume tokens, verify remaining
- Test rate limit: N requests pasan, N+1 es 429
- Test refill después de esperar
- Test headers de rate limit
- Test PII redacta contenido entre tags
- Test PII no modifica texto sin tags
- Test PII con múltiples tags en mismo texto
- Test PII con tags anidadas (no debe romperse)
- Test middleware rate limit integration
- Test middleware PII integration en logs
