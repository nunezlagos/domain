# Design: issue-33.1-rate-limit-per-org

## Contexto

El rate limit actual (`internal/auth/ratelimit/ratelimit.go` +
middleware) es GLOBAL. Un `rateLimiter` único con 120 req/min
compartido por TODOS los clientes. Esto es funcional para dev
local, pero en multi-tenant es un riesgo operacional: el primer
cliente con un script en loop te tira el servicio para todos.

La solución estándar: token bucket POR ORG, con un cache de
buckets in-memory (LRU eviction) y config per-org.

## Decisión arquitectónica

**Estrategia:** `golang.org/x/time/rate` con `sync.Map` keyed
por `orgID`, eviction LRU cada 10 min para buckets idle.

1. **Token bucket per-org:**
   ```go
   type OrgRateLimiter struct {
       buckets sync.Map  // map[string]*orgBucket
   }
   type orgBucket struct {
       limiter *rate.Limiter  // x/time/rate
       lastUsed time.Time
   }
   ```
   `OrgRateLimiter.Allow(orgID string) bool`:
   - `bucket := getOrCreate(orgID)`.
   - Update `lastUsed = now`.
   - Return `bucket.limiter.Allow()`.

2. **Config per-org (nueva tabla `org_rate_limits`):**
   ```sql
   CREATE TABLE org_rate_limits (
     organization_id UUID PRIMARY KEY REFERENCES organizations(id),
     rate_per_minute INT NOT NULL DEFAULT 1000,
     burst INT NOT NULL DEFAULT 2000,  -- 2x rate
     updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
   );
   ```
   Seed: para cada org existente, insertar default (1000/min, 2000
   burst).

3. **Default fallback:** si una org NO tiene entry, usar
   `RateLimitDefaultPerMinute = 1000` y `BurstDefault = 2000` del
   config global. Loggear "org X using default rate limit" solo la
   primera vez.

4. **Hot-reload de config:** el `runtimeconfig.Registry` (issue-27.3)
   refresca la tabla `org_rate_limits` cada 30s. El limiter
   chequea `lastRefresh` y re-carga si es viejo. Esto permite
   cambiar el rate sin restart.

5. **Eviction LRU:** goroutine que corre cada 10 min:
   ```go
   for _, entry := range buckets.Range() {
     if now.Sub(entry.lastUsed) > 1*time.Hour {
       buckets.Delete(entry.key)
     }
   }
   ```
   Para 50K orgs, la iteración es O(N) en microsegundos.
   Aceptable.

6. **Response headers** (en TODOS los responses, no solo 429):
   - `X-RateLimit-Limit: <rate_per_minute>`.
   - `X-RateLimit-Remaining: <tokens_left>` (compute vía
     `limiter.Tokens()`).
   - `X-RateLimit-Reset: <unix_ts cuando el bucket se llena>`.

7. **429 con retry-after:**
   - Status 429.
   - Header `Retry-After: <seg>` (compute: `time until next
     token = 60/rate segundos`).
   - Body: `ErrorResponse{Code: "rate_limited", Message: "...",
     Details: {Used, Limit, ResetAt}}`.

8. **Allowlist global** (no consume rate limit): `/health`,
   `/api/version`, `/api/v1/openapi.json`, `/auth/login`,
   `/auth/verify` (con rate limit propio e.g. 5/min por IP).

9. **Multi-pod future:** la interface es:
   ```go
   type RateLimiter interface {
       Allow(orgID string) (allowed bool, retryAfter time.Duration, info LimitInfo)
   }
   ```
   In-memory: `golang.org/x/time/rate`.
   Redis (futuro): `redis_rate` con sliding window.
   Mismo interface, swap por config.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Rate limit per-user (no per-org) | Más granular pero el problema es OPERACIONAL (un org = un cliente que paga). Per-org es la unidad correcta. |
| B | Postgres-backed counter (no in-memory) | Funciona cross-pod pero cada request es una query. Hot path. In-memory es 100x más rápido. |
| C | Cloudflare/CDN rate limit | No aplica: el server está detrás de Caddy directo, sin CDN. Y queremos control fino por org, no por IP. |
| D | "Planes" con tiers | El user fue explícito: no premium/Stripe/paywall. Config per-org, no per-plan. |

## Por qué in-memory + LRU + hot-reload gana

- **Performance:** token bucket es O(1) por check. Cero DB en hot
  path.
- **Operacional:** cambiar rate de un cliente es una UPDATE en
  `org_rate_limits`, no un deploy.
- **Memory-bounded:** LRU eviction previene OOM en
  multi-tenant con muchos clientes idle.
- **Swap-ready:** la interface permite migrar a Redis sin tocar
  el middleware.

## Detalle de implementación

- Migración: `migrations/000092_org_rate_limits.sql`.
- Seeder: `internal/seeds/org_rate_limits_seeder.go` (idempotente,
  skip-by-hash como el resto).
- `internal/auth/ratelimit/per_org.go`:
  - `OrgRateLimiter` con `sync.Map`.
  - `Allow(orgID) (allowed, retryAfter, info)`.
  - `Refresh(orgID) (limiter, error)` que lee la config de DB.
  - Goroutine de eviction LRU.
- `internal/auth/ratelimit/middleware.go`:
  - Wrap del middleware existente.
  - Si ruta en allowlist → skip.
  - Else: lookup principal orgID del context (post-auth), call
    `Allow`, populate headers, si denied → 429.

- Wire en `cmd/domain/main.go`: reemplazar el
  `rateLimiter` global por el `OrgRateLimiter`.

## Riesgos

- **R1:** In-memory NO se comparte entre pods. **Aceptable para
  MVP:** un solo pod por ahora. Documentar y planificar Redis
  para cuando escalen a 2+ pods (issue-26.x).
- **R2:** Hot-reload cada 30s = un cambio tarda 30s en aplicar.
  **Aceptable:** no es crítico. Si urge, se puede hacer
  `POST /admin/rate-limit/refresh` con auth de admin.
- **R3:** `golang.org/x/time/rate` no es fair entre muchos
  buckets bajo carga extrema. **Aceptable:** nuestros
  volúmenes son bajos (<1000 RPS). Si crece, swap por Redis
  sliding window.
