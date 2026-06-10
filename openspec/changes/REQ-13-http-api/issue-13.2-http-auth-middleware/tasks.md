# Tasks: issue-13.2-http-auth-middleware

## Backend

- [ ] Implementar `extractBearerToken()` del header Authorization
- [ ] Implementar `AuthMiddleware` con validación contra tabla api_keys
- [ ] Implementar caché de API keys con TTL (sync.Map + cleanup goroutine)
- [ ] Implementar `RBACMiddleware` que verifica permisos por entidad+acción
- [ ] Definir roles base: admin, editor, viewer con sus permisos
- [ ] Implementar `RateLimiter` con sliding window counters en memoria
- [ ] Implementar sharding de rate limit buckets para concurrencia
- [ ] Implementar `RequestLogger` con duración, status, método, path, api_key_id, ip
- [ ] Implementar sanitización de Authorization header en logs
- [ ] Implementar CORS middleware configurable
- [ ] Configurar middleware chain en router principal
- [ ] Excluir /api/v1/health de auth
- [ ] Manejar edge cases: token vacío, malformado, expires_at vencido, is_active=false

## Frontend

- [ ] N/A (API pura)

## Tests

- [ ] Test unitario: extracción Bearer token
- [ ] Test unitario: auth middleware sin token → 401
- [ ] Test unitario: auth middleware con token inválido → 401
- [ ] Test unitario: auth middleware con token expirado → 401
- [ ] Test unitario: auth middleware con token desactivado → 401
- [ ] Test unitario: RBAC permisos correctos → 200, incorrectos → 403
- [ ] Test unitario: rate limiter dentro del límite → pasa, excede → 429
- [ ] Test unitario: rate limiter reset después de ventana
- [ ] Test unitario: health endpoint no requiere auth
- [ ] Test unitario: CORS headers presentes
- [ ] Test unitario: sanitización de tokens en logs
- [ ] Sabotaje: eliminar auth middleware → test CRUD sin token detecta

## Cierre

- [ ] Verificación manual: curl con/ sin token contra endpoints
- [ ] Suite verde: `go test ./internal/api/...`
- [ ] Rate limit test con 101 requests en 60s
