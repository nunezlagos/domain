# Tasks: issue-02.5-rate-limit-pii

## Backend

### Rate Limiting
- [x] Crear `internal/ratelimit/bucket.go` con TokenBucket struct
- [x] Crear `internal/ratelimit/limiter.go` con RateLimiter (sync.Map, cleanup)
- [x] Implementar `Allow(key string) (ok bool, retryAfter time.Duration)`
- [x] Crear middleware `RateLimitMiddleware` en `internal/api/middleware/ratelimit.go`
- [x] Agregar headers rate limit a responses
- [x] Agregar configuración via `DOMAIN_RATE_LIMIT_REQUESTS` y `DOMAIN_RATE_LIMIT_WINDOW`
- [x] Goroutine de cleanup cada 10 minutos
- [x] Registrar middleware antes de auth middleware

### PII Redaction
- [x] Crear `internal/sanitize/pii.go` con `Sanitize()` y `SanitizeJSON()`
- [x] Integrar Sanitize en storage layer (observations, prompts, knowledge_docs)
- [x] Crear middleware PII para logging de request bodies
- [x] Crear hook para logger (Logrus/zerolog) que sanitiza mensajes
- [x] Agregar test que verifica PII no se almacena en DB

## Tests

- [x] Test unitario: token bucket consume hasta límite
- [x] Test unitario: Allow retorna false después del límite
- [x] Test unitario: refill después de esperar
- [x] Test unitario: buckets independientes por key
- [x] Test unitario: headers rate limit correctos
- [x] Test middleware: 429 después de exceder límite
- [x] Test unitario: Sanitize reemplaza contenido private
- [x] Test unitario: Sanitize sin tags no modifica
- [x] Test unitario: Sanitize múltiples tags
- [x] Test unitario: SanitizeJSON con strings JSON
- [x] Test integración: storage sanitiza antes de INSERT
- [x] Sabotaje: no llamar Sanitize en storage → confirmar que PII se almacena → restaurar
- [x] Sabotaje: cambiar regex greedy `.*` → confirmar que test multi-tag falla → restaurar

## Cierre

- [x] Verificación manual: exceder rate limit, verificar 429 y headers
- [x] Verificación manual: enviar `<private>test</private>` y verificar redacción
- [x] Suite verde
