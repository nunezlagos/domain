# Tasks: issue-33.1-rate-limit-per-org

## Backend

- [ ] **T1**: Crear migración `migrations/000092_org_rate_limits.sql`:
  - Tabla `org_rate_limits` con schema de design.md.
  - Default 1000/min, 2000 burst.

- [ ] **T2**: Seeder `internal/seeds/org_rate_limits_seeder.go`:
  - Idempotente: para cada org existente, INSERT ON CONFLICT DO
    NOTHING con los defaults.
  - Registrado en `seeds.Registry` (mismo patrón que los otros).

- [ ] **T3**: Crear `internal/auth/ratelimit/per_org.go`:
  - `type OrgRateLimiter struct { db *pgxpool.Pool; buckets
    sync.Map; config Config }`.
  - `type Config struct { DefaultRatePerMinute int; DefaultBurst int;
    EvictionIntervalMinutes int; IdleEvictionAfter time.Duration
    }`.
  - `Allow(orgID string) (allowed bool, retryAfter time.Duration,
    info LimitInfo)`.
  - `LimitInfo` con `Limit, Remaining, ResetAt`.
  - `getOrCreateBucket(orgID) *orgBucket`: lee de `buckets` o crea
    con `rate.NewLimiter(rate.Every(time.Minute/time.Duration(ratePerMin)),
    burst)`.
  - Goroutine de eviction LRU (corre cada 10 min default).

- [ ] **T4**: Refresh desde DB: cuando un bucket no existe para
  una org, query a `org_rate_limits`. Cachear resultado in-memory
  por 30s (evitar query por cada request). Hot-reload via
  `runtimeconfig.Registry`.

- [ ] **T5**: Refactor del middleware actual
  (`internal/api/middleware/rate_limit.go` o similar) para usar
  `OrgRateLimiter`:
  - Lookup `principal.OrganizationID` del context (post-auth).
  - Si la ruta está en `allowlist` (health, openapi, etc) → skip.
  - Else: `allowed, retryAfter, info := limiter.Allow(orgID)`.
  - Si denied: 429 con `Retry-After` + body.
  - Si allowed: set `X-RateLimit-*` headers y continue.

- [ ] **T6**: Wire en `cmd/domain/main.go`: reemplazar el
  `rateLimiter` global con `OrgRateLimiter`. Configurar defaults
  desde `config.Config` (nuevos campos `RateLimitPerOrgDefault`,
  `RateLimitBurstDefault`).

- [ ] **T7**: Admin endpoint `POST /api/v1/admin/rate-limit/{orgID}`
  para override manual (con auth de admin). Útil para
  "este cliente está atacando, bajarlo a 10/min AHORA".

## Tests

- [ ] **T-unit-1**: `TestOrgRateLimiter_AllowsUpToLimit**` — 100
  requests rápidas con orgID X → las 100 primeras pasan.
- [ ] **T-unit-2**: `TestOrgRateLimiter_DeniesOverLimit**` — 1500
  requests con rate=1000/min → las primeras ~1000 (más burst)
  pasan, el resto 429.
- [ ] **T-unit-3**: `TestOrgRateLimiter_PerOrgIsolation**` — org A
  hace 1500 requests (excede) → org B hace 100 → org B no es
  afectada. Ambas verificadas independientemente.
- [ ] **T-unit-4**: `TestOrgRateLimiter_LRUEviction**` — crear
  buckets para 10K orgs sin uso → esperar eviction interval →
  memory <threshold.
- [ ] **T-unit-5**: `TestOrgRateLimiter_DefaultFallback**` — org
  sin entry en `org_rate_limits` → usa default 1000/min.
- [ ] **T-e2e-1**: `TestMiddleware_Headers**` — request OK → response
  tiene `X-RateLimit-Limit`, `X-RateLimit-Remaining`,
  `X-RateLimit-Reset`.
- [ ] **T-e2e-2**: `TestMiddleware_429**` — request que excede →
  response 429 con `Retry-After: <seg>` y body JSON con
  `error_code: rate_limited`.
- [ ] **T-e2e-3**: `TestMiddleware_AllowlistSkips**` — request a
  `/health` no consume tokens (verificar que `Allow` no se llamó).
- [ ] **T-sabotaje**: Comentar el lookup `orgID` del context y
  hardcodear un único bucket global (sabotaje) → test unit-3
  DEBE FALLAR (org B es afectada por exceso de A) → restaurar
  lookup → test verde. Documentar sabotaje en commit body.
