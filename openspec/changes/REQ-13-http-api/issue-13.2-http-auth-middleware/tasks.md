# Tasks: issue-13.2-http-auth-middleware

## Backend

- [x] Implementar `extractBearerToken()` del header Authorization
- [x] Implementar `AuthMiddleware` con validación contra tabla api_keys
- [x] Implementar caché de API keys con TTL (sync.Map + cleanup goroutine)
- [x] Implementar `RBACMiddleware` que verifica permisos por entidad+acción
- [x] Definir roles base: admin, editor, viewer con sus permisos
- [x] Implementar `RateLimiter` con sliding window counters en memoria
- [x] Implementar sharding de rate limit buckets para concurrencia
- [x] Implementar `RequestLogger` con duración, status, método, path, api_key_id, ip
- [x] Implementar sanitización de Authorization header en logs
- [x] Implementar CORS middleware configurable
- [x] Configurar middleware chain en router principal
- [x] Excluir /api/v1/health de auth
- [x] Manejar edge cases: token vacío, malformado, expires_at vencido, is_active=false

## Frontend

- [x] N/A (API pura)

## Tests

- [x] Test unitario: extracción Bearer token
- [x] Test unitario: auth middleware sin token → 401
- [x] Test unitario: auth middleware con token inválido → 401
- [x] Test unitario: auth middleware con token expirado → 401
- [x] Test unitario: auth middleware con token desactivado → 401
- [x] Test unitario: RBAC permisos correctos → 200, incorrectos → 403
- [x] Test unitario: rate limiter dentro del límite → pasa, excede → 429
- [x] Test unitario: rate limiter reset después de ventana
- [x] Test unitario: health endpoint no requiere auth
- [x] Test unitario: CORS headers presentes
- [x] Test unitario: sanitización de tokens en logs
- [x] Sabotaje: eliminar auth middleware → test CRUD sin token detecta

## Cierre

- [x] Verificación manual: curl con/ sin token contra endpoints
- [x] Suite verde: `go test ./internal/api/...`
- [x] Rate limit test con 101 requests en 60s
