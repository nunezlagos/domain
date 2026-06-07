# HU-26.7-cache-invalidation-patterns

**Origen:** `REQ-26-horizontal-scalability`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** desarrollador
**Quiero** patrón uniforme para invalidación de caches in-memory cross-pod usando Postgres LISTEN/NOTIFY
**Para** que cualquier feature que cachee algo invalide correctamente sin duplicar mecanismo

## Casos de uso

- Cache permisos custom_roles (HU-02.8)
- Cache policies (HU-01.8)
- Cache configs MCP tools (HU-12.6)
- Cache plans + custom_limits (HU-21.3)
- Cache de model_registry pricing
- Cache LRU de agent definitions

## Criterios de aceptación

### Escenario 1: Helper genérico

```gherkin
Dado que existe package `internal/cache/distributed/`
Cuando wrap una cache local con `WithInvalidation(cache, channel)`
Entonces cualquier UPDATE/DELETE en la entidad publica NOTIFY al channel
Y todos los pods reciben + invalidan su cache local
Y se logea métrica
```

### Escenario 2: Convención channel naming

```gherkin
Dado que channel `cache_invalidate_<entity>`
Cuando entity es `custom_roles`
Entonces channel = `cache_invalidate_custom_roles`
Y payload = `{operation: insert|update|delete, id: "...", organization_id: "..."}`
```

### Escenario 3: Trigger Postgres helper

```gherkin
Dado que existe función helper `create_cache_invalidation_trigger(table)`
Cuando se ejecuta para tabla `custom_roles`
Entonces se crea trigger AFTER INSERT OR UPDATE OR DELETE
Y NOTIFY se publica con payload JSON
Y la migration uses ese helper consistentemente
```

### Escenario 4: Reconnect

```gherkin
Dado que connection listener cae temporalmente
Cuando reconecta
Entonces invalida todo el cache (safe default; refetch on demand)
Y se publica métrica de reconnect
```

### Escenario 5: Throughput

```gherkin
Dado que pocos pods reciben muchos NOTIFY/s (high churn)
Cuando se procesa
Entonces dedupe in-memory window 100ms (batch multiple NOTIFY del mismo id)
```

### Escenario 6: Métricas

```gherkin
Dado que cada invalidación
Entonces publica:
  - `domain_cache_invalidations_total{channel,operation}`
  - `domain_cache_listener_reconnects_total`
  - `domain_cache_lag_seconds` (delta entre NOTIFY publish y consumer process)
```

## Análisis breve

- **Qué pide:** helper genérico + trigger Postgres + reconnect + métricas + convención
- **Esfuerzo:** S
- **Riesgos:** NOTIFY payload limit 8000 bytes; throughput; conn-level (no pgbouncer txn)
