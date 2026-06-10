# Tasks: issue-02.5-rate-limit-pii

## Backend

### Rate Limiting
- [ ] Crear `internal/ratelimit/bucket.go` con TokenBucket struct
- [ ] Crear `internal/ratelimit/limiter.go` con RateLimiter (sync.Map, cleanup)
- [ ] Implementar `Allow(key string) (ok bool, retryAfter time.Duration)`
- [ ] Crear middleware `RateLimitMiddleware` en `internal/api/middleware/ratelimit.go`
- [ ] Agregar headers rate limit a responses
- [ ] Agregar configuración via `DOMAIN_RATE_LIMIT_REQUESTS` y `DOMAIN_RATE_LIMIT_WINDOW`
- [ ] Goroutine de cleanup cada 10 minutos
- [ ] Registrar middleware antes de auth middleware

### PII Redaction
- [ ] Crear `internal/sanitize/pii.go` con `Sanitize()` y `SanitizeJSON()`
- [ ] Integrar Sanitize en storage layer (observations, prompts, knowledge_docs)
- [ ] Crear middleware PII para logging de request bodies
- [ ] Crear hook para logger (Logrus/zerolog) que sanitiza mensajes
- [ ] Agregar test que verifica PII no se almacena en DB

## Tests

- [ ] Test unitario: token bucket consume hasta límite
- [ ] Test unitario: Allow retorna false después del límite
- [ ] Test unitario: refill después de esperar
- [ ] Test unitario: buckets independientes por key
- [ ] Test unitario: headers rate limit correctos
- [ ] Test middleware: 429 después de exceder límite
- [ ] Test unitario: Sanitize reemplaza contenido private
- [ ] Test unitario: Sanitize sin tags no modifica
- [ ] Test unitario: Sanitize múltiples tags
- [ ] Test unitario: SanitizeJSON con strings JSON
- [ ] Test integración: storage sanitiza antes de INSERT
- [ ] Sabotaje: no llamar Sanitize en storage → confirmar que PII se almacena → restaurar
- [ ] Sabotaje: cambiar regex greedy `.*` → confirmar que test multi-tag falla → restaurar

## Cierre

- [ ] Verificación manual: exceder rate limit, verificar 429 y headers
- [ ] Verificación manual: enviar `<private>test</private>` y verificar redacción
- [ ] Suite verde
