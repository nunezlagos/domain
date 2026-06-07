# Proposal: HU-13.2-http-auth-middleware

## Intención

Implementar un middleware chain que proteja todas las rutas API con autenticación Bearer token, autorización RBAC, rate limiting y request logging. El único endpoint público es `/api/v1/health`.

## Scope

**Incluye:**
- Middleware de autenticación: extrae Bearer token del header Authorization, valida contra tabla api_keys
- Middleware de autorización: verifica permisos basados en rol + entidad + acción
- Middleware de rate limiting: sliding window por API key, configurable
- Middleware de request logging: duración, status code, API key ID, IP
- CORS middleware configurable (orígenes, métodos, headers)
- Excepción para health endpoint
- Cache de API keys en memoria con TTL para reducir latencia

**Excluye:**
- Autenticación OAuth2 / SSO (futuro)
- IP-based allow/deny lists (futuro)
- Audit logging detallado de cada request (se cubre en REQ-02)

## Enfoque técnico

**Middleware chain (orden de ejecución):**
```
Request → CORS → RequestLogger → RateLimiter → Auth → RBAC → Handler
```

**Auth middleware:**
```go
func AuthMiddleware(store APIKeyStore) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. Skip /health
            if r.URL.Path == "/api/v1/health" {
                next.ServeHTTP(w, r)
                return
            }
            // 2. Extract Bearer token
            token := extractBearerToken(r)
            // 3. Validate against store (with cache)
            key, err := store.ValidateAndGet(r.Context(), token)
            // 4. Inject API key info into context
            ctx := context.WithValue(r.Context(), CtxAPIKey, key)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

**RBAC middleware:**
```go
func RBACMiddleware(permissions map[string][]string) func(http.Handler) http.Handler {
    // Lee API key del context
    // Extrae entidad y acción de la ruta (método HTTP + path pattern)
    // Verifica: key.Role.Permissions contiene "entity:action"
    // Si no → 403
}
```

**Rate limiter: sliding window counters**
```go
type SlidingWindow struct {
    mu       sync.RWMutex
    buckets  map[string]*Window // key: api_key_id
    limit    int
    window   time.Duration
}
```

**API key cache:**
- Map concurrente con TTL de 5 minutos
- Invalidación on write (cuando se crea/actualiza/desactiva una key)
- Fallback a DB query si cache miss

## Riesgos

| Riesgo | Mitigación |
|--------|------------|
| Auth middleware overhead en cada request | Cache de API keys con TTL, rate limiter en memoria (sin DB) |
| Race condition en rate limiter counters | sync.RWMutex + sharded buckets para high throughput |
| Tokens en logs (seguridad) | Sanitizar header Authorization antes de loguear |
| CORS mal configurado expone la API | Config default restrictiva, solo orígenes explicitos |

## Testing

- Unit tests para cada middleware de forma aislada
- Test de integración: middleware chain completo
- Test de rate limiting con time-warp (reloj mockeado)
- Test de seguridad: tokens en logs sanitizados
- Sabotaje: sacar auth check de una ruta → test detecta
