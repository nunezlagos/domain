# Design: HU-13.2-http-auth-middleware

## Decisión arquitectónica

**Middleware chain pattern:** Cada middleware es un `func(http.Handler) http.Handler` que se compone linealmente. El orden es crítico: Logger primero (captura todo), RateLimiter antes que Auth (protege contra ataques de autenticación), Auth antes que RBAC (necesitamos saber quién es antes de verificar permisos).

```
Request
  │
  ▼
┌──────────────┐
│  CORS        │ ─── OPTIONS → 200 con headers
└──────┬───────┘
       ▼
┌──────────────┐
│  Logger      │ ─── start timer, wrap ResponseWriter
└──────┬───────┘
       ▼
┌──────────────┐
│  RateLimiter │ ─── check counter, 429 si excede
└──────┬───────┘
       ▼
┌──────────────┐
│  Auth        │ ─── extract Bearer, validate, inject ctx
└──────┬───────┘
       ▼
┌──────────────┐
│  RBAC        │ ─── match route → check permission → 403
└──────┬───────┘
       ▼
    Handler
```

**API Key model simplificado:**
```go
type APIKey struct {
    ID          string    `json:"id"`
    KeyPrefix   string    `json:"key_prefix"`   // primeros 8 chars para identificar
    KeyHash     string    `json:"-"`             // bcrypt hash del key completo
    Name        string    `json:"name"`
    Role        string    `json:"role"`           // admin, editor, viewer
    Permissions []string  `json:"permissions"`    // ["observations:read", "observations:write", ...]
    ProjectID   string    `json:"project_id,omitempty"` // scope a proyecto
    IsActive    bool      `json:"is_active"`
    ExpiresAt   *time.Time `json:"expires_at,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
}
```

**Permission schema:**
- `<entity>:<action>` donde action ∈ {create, read, update, delete, list}
- Rol admin → todas las acciones en todas las entidades
- Rol editor → read + write en entidades específicas
- Rol viewer → read-only

**Rate limit config:**
```yaml
rate_limit:
  default: 100/minute
  burst: 20
  by_endpoint:
    POST /api/v1/observations: 30/minute
    GET /api/v1/health: unlimited
```

## Alternativas descartadas

1. **JWT-based auth:** Sobredimensionado para API key simple. JWT sería útil si hubiera OAuth2/SSO. Con API keys + bcrypt es más simple y seguro.
2. **Redis para rate limiting:** Dependencia externa adicional. Sliding window en memoria es suficiente para instancia única. Escalar a multi-instancia requiere Redis (futuro).
3. **Middleware por ruta:** Configurar auth en cada route handler vs global. Global es más seguro (nadie se olvida de agregarlo).

## Diagrama

```
┌──────────────────────────────────────────────────────────────┐
│                      API Server                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  Router (chi)                                          │  │
│  │  ├── CORS                                              │  │
│  │  ├── RequestLogger                                     │  │
│  │  ├── RateLimiter                                       │  │
│  │  ├── AuthMiddleware                                    │  │
│  │  ├── RBACMiddleware                                    │  │
│  │  └── Subrouters                                        │  │
│  │       ├── /api/v1/observations → CRUDHandlers          │  │
│  │       ├── /api/v1/users → CRUDHandlers                 │  │
│  │       └── ...                                          │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  API Key Cache (sync.Map + TTL)                        │  │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐                  │  │
│  │  │ key_abc │ │ key_def │ │ key_ghi │  ...              │  │
│  │  └─────────┘ └─────────┘ └─────────┘                  │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  Rate Limit Buckets (sharded sync.Map)                  │  │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐                  │  │
│  │  │ key_abc │ │ key_def │ │ key_ghi │  ...              │  │
│  │  │ count:23│ │ count:45│ │ count:12│                  │  │
│  │  └─────────┘ └─────────┘ └─────────┘                  │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

## TDD plan

1. **Red:** Test `TestAuthMiddleware_NoToken_Returns401`
2. **Green:** Auth middleware mínimo que rechaza requests sin header
3. **Refactor:** Agregar cache, extraer token extraction
4. **Iterar:** RBAC → RateLimiter → Logger → CORS
5. **Sabotaje:** Eliminar auth check de una ruta → test detecta

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Cache de API keys stale | TTL 5 min + invalidación forzada on DB write |
| Rate limiter consume mucha memoria con muchas keys | Sharding por hash de api_key_id, cleanup de buckets inactivos cada 10 min |
| Logging captura tokens en headers | Sanitize middleware que reemplaza Authorization antes de loguear |
| CORS demasiado permisivo | Default: deny all. Solo orígenes explicitos en config |
