# issue-12.6-mcp-tool-resilience

**Origen:** `REQ-12-mcp-server`
**Prioridad tentativa:** alta
**Tipo:** feature + hardening

## Historia de usuario

**Como** agente Claude/Cline consumiendo MCP tools de Domain
**Quiero** que cada tool tenga timeout, circuit breaker, cache local last-known-good y respuestas degraded explícitas
**Para** que la DB caída o un endpoint slow no rompa la sesión del agente

## Patrones aplicados a TODOS los MCP tools

| patrón | aplicación |
|--------|-----------|
| Timeout per-tool | configurable, default 5s; query tools 2s; write tools 10s |
| Circuit breaker | tripped tras 5 errores consecutivos en 30s; reset try cada 60s |
| Cache local LRU | last-known-good por (tool, args_hash) TTL 5min para tools idempotentes |
| Graceful degraded | si DB lenta/caída + cache hit → return cached con flag `{degraded: true, cached_at: T}` |
| Fail-fast | si circuit OPEN + no cache → return error tipado, NO esperar timeout |
| Retry interno | solo si error transitorio (5xx, timeout); max 1 retry con backoff 100ms |
| Observability | métricas + traces + logs estructurados (issue-17.*) per tool |

## Criterios de aceptación

### Escenario 1: Tool con timeout strict

```gherkin
Dado que tool `domain_mem_search` tiene `timeout: 2s`
Cuando la query DB tarda 3s
Entonces el tool aborta a los 2s
Y respuesta error `{"code": "tool_timeout", "tool": "domain_mem_search", "timeout_ms": 2000}`
Y métrica `domain_mcp_tool_timeouts_total{tool}` incrementa
Y context cancellation propagado a DB query
```

### Escenario 2: Circuit breaker

```gherkin
Dado que tool tuvo 5 errores consecutivos en últimos 30s
Cuando llega request 6to
Entonces circuit OPEN → fail-fast con error `{"code": "circuit_open", "tool": "X", "retry_after_seconds": 60}`
Y NO se intenta query DB (ahorra recursos)
Y métrica `domain_mcp_circuit_state{tool}` = "open"
Y tras 60s se intenta half-open: 1 request prueba
Y si éxito → CLOSED; si error → OPEN reset 60s
```

### Escenario 3: Cache local con graceful degraded

```gherkin
Dado que tool `domain_policy_get(slug="db")` tiene cache LRU TTL 5min
Y DB caída
Y el cache tiene entry de hace 3min
Cuando agente invoca el tool
Entonces se devuelve cached con:
  ```json
  {
    "data": {...policy...},
    "degraded": true,
    "cached_at": "2026-06-07T11:57:00Z",
    "age_seconds": 180
  }
  ```
Y métrica `domain_mcp_degraded_responses_total{tool}` incrementa
Y log warn "serving stale from cache; DB unhealthy"
```

### Escenario 4: Cache miss + DB caída

```gherkin
Dado que cache vacío para esa args_hash
Y DB caída
Cuando invoca tool
Entonces error `{"code": "service_unavailable", "tool": "X", "reason": "no_cache_and_db_down"}`
Y NO se sirve cached desactualizado
```

### Escenario 5: Retry transitorio interno

```gherkin
Dado que primer attempt error 5xx o context deadline
Cuando retry policy = 1 con backoff 100ms
Entonces intenta una segunda vez
Y si segunda exitosa → success normal
Y si segunda falla → error final + retry NO se hace una 3ra vez
Y métrica `domain_mcp_tool_retries_total{tool}` incrementa
```

### Escenario 6: Write tools NO cachean

```gherkin
Dado que tool `domain_mem_save` es write
Cuando se invoca con DB caída
Entonces error inmediato `{"code": "service_unavailable"}`
Y NO hay cache local (write tools tienen `cacheable: false` en config)
Y NO hay retry transitorio en writes (riesgo de doble-write)
Y idempotency-key del caller debe manejar retries si quiere
```

### Escenario 7: Métricas y observabilidad

```gherkin
Dado que cada tool ejecuta
Cuando se procesa
Entonces se emiten:
  | métrica                                | tipo      | labels                |
  | domain_mcp_tool_calls_total            | counter   | tool, status, source  |
  | domain_mcp_tool_duration_seconds       | histogram | tool                  |
  | domain_mcp_tool_timeouts_total         | counter   | tool                  |
  | domain_mcp_circuit_state               | gauge     | tool (0=closed,1=open,2=half) |
  | domain_mcp_cache_hits_total            | counter   | tool, mode (fresh|degraded) |
  | domain_mcp_degraded_responses_total    | counter   | tool                  |
Y trace span per tool call con attrs tool.name, tool.status, tool.cached
```

### Escenario 8: Tool config en BD

```gherkin
Dado que existe tabla `mcp_tool_configs` con per-tool:
  (slug, timeout_ms, cacheable BOOL, cache_ttl_seconds, retry_count, circuit_threshold, circuit_reset_seconds)
Cuando boot MCP server
Entonces carga config desde BD
Y admin puede ajustar via API `/admin/mcp/tools/:slug/config`
Y cambio reload sin restart vía LISTEN/NOTIFY
```

### Escenario 9: Tests adversariales

```gherkin
Dado que existe scenario test "DB timeout"
Cuando inyectamos slow query +5s
Entonces cada tool maneja según su config:
  - mem_search 2s timeout → fail-fast 2s
  - policy_get con cache hit → serve degraded
  - mem_save → fail-fast 10s (no cache write)
```

## Análisis breve

- **Qué pide:** middleware MCP genérico timeout+cb+cache+retry + config en BD + métricas + tests adversariales
- **Esfuerzo:** L
- **Riesgos:** complejidad debugging cuando cache vs fresh diverge; circuit breaker false positives; tools de write con retry causando duplicates
