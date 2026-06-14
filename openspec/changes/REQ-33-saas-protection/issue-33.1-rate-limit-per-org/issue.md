# issue-33.1-rate-limit-per-org

**Origen:** `REQ-33-saas-protection`
**Prioridad tentativa:** media
**Tipo:** feature (operational)

## Historia de usuario

**Como** operador del VPS multi-tenant
**Quiero** que el rate limit del harness MCP sea POR ORG, no global
**Para** que un cliente abusivo (intencional o por bug) no pueda afectar el servicio del resto de clientes

## Criterios de aceptación

### Escenario 1: Rate limit per-org activado

```gherkin
Dado que org A tiene config `rate_limit_per_minute=1000`
Y org B tiene config `rate_limit_per_minute=500`
Cuando org A hace 1500 requests en 1 minuto
Entonces org A recibe 429 después del request 1000
Y org B sigue respondiendo normal (no afectada)
Y el response de A tiene header `X-RateLimit-Limit: 1000`, `X-RateLimit-Remaining: 0`, `X-RateLimit-Reset: <ts>`
```

### Escenario 2: Default global razonable

```gherkin
Dado que una org nueva sin config explícita
Cuando hace requests
Entonces el rate limit es 1000/min (default razonable)
Y el log indica: "org X using default rate limit 1000/min"
```

### Escenario 3: Token bucket in-memory con eviction

```gherkin
Dado que el server corre 24h
Y 50K orgs distintas hicieron al menos 1 request
Cuando el server chequea memoria
Entonces el cache de token buckets NO crece sin límite
Y hay eviction LRU cada 10 minutos para buckets sin uso en 1h
Y memoria <500MB para el cache de rate limit
```

### Escenario 4: Storage persistente (opcional, para accuracy cross-pod)

```gherkin
Dado que el server corre con múltiples pods (futuro, issue-26.x)
Y un cliente con org A hace requests distribuidas en 2 pods
Cuando se cuenta el rate limit
Entonces el conteo es COMPARTIDO entre pods (no 2000/min total por org)
Y el storage es Redis (o tabla en Postgres con counter)
Y el modo in-memory (single pod) sigue siendo el default para dev
```

### Escenario 5: 429 con retry-after header

```gherkin
Dado que org A excedió el rate limit
Cuando hace un nuevo request
Entonces el response es 429 Too Many Requests
Y header `Retry-After: <segundos hasta el próximo token disponible>`
Y body JSON con `error_code: "rate_limited", error_message: "..."
, current_usage: {used, limit, reset_at}`
```

### Escenario 6: Bypass para allowlist (health, openapi)

```gherkin
Dado que el rate limit per-org está activo
Cuando se hace `GET /health`, `GET /api/v1/openapi.json`, `GET /api/version`
Entonces NO se cuenta contra el rate limit
Y estas rutas son globales (no per-org, no per-user)
```

### Escenario 7: Sabotaje — rate limit sigue siendo global

```gherkin
Dado que org A hace 100 requests rápidos
Y org B hace 100 requests rápidos
Y el código tiene un bug (sabotaje) que el rate limit sigue siendo global
Cuando se excede el límite global (e.g. 120/min)
Entonces TANTO A como B reciben 429, no solo A
Y el test e2e que assserta "B no afectada por exceso de A" DEBE FALLAR
Cuando restauro el bucket per-org
Entonces el test verde
```

### Escenario 8: Edge case — burst > rate por pocos segundos

```gherkin
Dado que org A tiene rate 60/min (1/seg promedio)
Y el bucket size es 2x rate = 120 (permite bursts de 120 instantáneos)
Cuando A hace 120 requests en 100ms
Entonces las 120 pasan (bucket se vacía)
Y la request 121 (1ms después) recibe 429
Y el bucket se rellena a 1/seg continuamente
```

## Notas

- Implementación con `golang.org/x/time/rate` (token bucket
  estándar, in-memory). Para multi-pod (futuro), swap por Redis.
- El `Principal.OrganizationID` ya está disponible en el context
  post-auth (issue-02.5). El middleware solo necesita leerlo.
- NO hay "tiers" ni "planes". La config es per-org, no per-plan.
  El user fue explícito: "no premium / Stripe / paywall".
