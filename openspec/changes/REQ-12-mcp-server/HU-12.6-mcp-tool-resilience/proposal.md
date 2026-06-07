# Proposal: HU-12.6-mcp-tool-resilience

## Intención

Middleware genérico aplicado a TODOS los MCP tools con timeout, circuit breaker, cache LRU last-known-good, retry transitorio, graceful degraded, configurable per-tool desde BD. Hace los tools de Domain production-grade aun cuando la DB hipo.

## Scope

**Incluye:**
- Package `internal/mcp/resilience/` con middleware composable
- Tabla `mcp_tool_configs` editable runtime
- Patrones: timeout, circuit breaker (gobreaker o equivalente), LRU cache (hashicorp/golang-lru), retry transitorio
- Métricas Prometheus per tool
- Tracing spans
- Tests con DB sabotaje (slow query, drop conn)
- LISTEN/NOTIFY config reload

**No incluye:**
- Bulkhead (separate goroutine pools per tool) — futuro
- Distributed circuit breaker cross-pod — single-pod state es suficiente

## Enfoque técnico

1. Tools registrados con metadata (idempotent, cacheable, timeout, etc.)
2. Middleware chain: tracing → metrics → cache check → circuit → timeout → retry → handler
3. Cache key: hash(tool_name + canonical_args_json)
4. Circuit breaker per tool (sonyflake/gobreaker)
5. Config hot-reload via LISTEN/NOTIFY

## Riesgos

- Cache stale en degraded: documentar bien, `degraded: true` flag explícito
- Write tools: explicit `cacheable: false` + sin retry (HU-13.4 idempotency-key cubre del lado caller)
- Cardinality métrica: tool labels acotados (~30)

## Testing

- Timeout abort
- Circuit breaker OPEN tras 5 errores
- Cache hit fresh
- Cache hit degraded
- Cache miss + DB down → 503
- Retry transitorio
- Write tool sin retry
- Config reload via NOTIFY
- DB sabotaje suite
