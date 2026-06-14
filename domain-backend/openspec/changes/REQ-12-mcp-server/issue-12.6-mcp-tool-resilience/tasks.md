# Tasks: issue-12.6-mcp-tool-resilience

> Decisión de producto 2026-06-10 (MCP-first): domain-mcp es un binario stdio
> local de sesión única. Config de budgets en BD + NOTIFY reload, cache LRU
> con degraded responses y endpoint admin son over-engineering para ese
> deployment — quedan DIFERIDOS. La resiliencia real implementada vive en
> ResilientWrapper (rate limit + retry + circuit breaker), sin deps nuevas
> (gobreaker/golang-lru innecesarios para esta escala).

- [ ] **mr-001**: Migration mcp_tool_configs + NOTIFY → DIFERIDO (budgets en código; binario stdio no comparte config entre pods)
- [x] **mr-002**: Middleware de resiliencia → ResilientWrapper en internal/mcp/server/resilience.go (rate limit + retry + CB)
- [x] **mr-003**: Deps → sin deps nuevas por diseño (token bucket + CB propios, ~80 líneas; gobreaker/lru innecesarios)
- [ ] **mr-004**: Cache key hashing → DIFERIDO con cache (mr-007)
- [x] **mr-005**: Circuit breaker per-tool → cbState (CBThreshold fallos consecutivos → open CBCooldown, half-open implícito, fallo en half-open re-abre directo) — 2026-06-11
- [x] **mr-006**: Retry classifier → isTransient/isTransientResult (connection reset, broken pipe, i/o timeout, deadline, 503) vs permanentes sin retry
- [ ] **mr-007**: Degraded response desde cache → DIFERIDO (sin cache; el agente recibe error claro y decide)
- [ ] **mr-008**: LISTEN/NOTIFY config reload → DIFERIDO con mr-001
- [ ] **mr-009**: Métricas Prometheus → DIFERIDO (binario stdio sin endpoint /metrics; las métricas viven en el server HTTP)
- [ ] **mr-010**: Tracing spans → DIFERIDO (ídem mr-009)
- [ ] **mr-011**: Endpoint admin /admin/mcp/tools → DIFERIDO con mr-001
- [x] **mr-012**: Middleware aplicado a TODOS los tools → Tools() wrappea cada handler; defaultBudget 120/min + CB(5, 30s); mutations 60/min — 2026-06-11
- [ ] **mr-013**: Seed mcp_tool_configs → DIFERIDO con mr-001
- [ ] **test-001**: Timeout abort → DIFERIDO (timeouts los maneja el context del cliente MCP; deadline exceeded sí se clasifica transient)
- [x] **test-002**: CB OPEN tras N errores → TestCircuitBreaker_OpensAfterThreshold (handler NO invocado con breaker abierto) — 2026-06-11
- [ ] **test-003**: Cache fresh hit → diferido con mr-007
- [ ] **test-004**: Cache degraded → diferido con mr-007
- [ ] **test-005**: Cache miss + DB down → diferido con mr-007
- [x] **test-006**: Retry transitorio → TestResilientWrapper_Retry_OnTransient (éxito en attempt 3)
- [x] **test-007**: Error permanente sin retry → TestResilientWrapper_Retry_NonTransientNoRetry
- [ ] **test-008**: Config reload NOTIFY → diferido con mr-001/mr-008
- [x] **test-009**: Recovery suite → TestCircuitBreaker_HalfOpenRecovery (clock inyectado, éxito resetea) + TestCircuitBreaker_HalfOpenFailureReopens (1 fallo re-abre) — 2026-06-11
- [x] **test-sabotaje**: TestSabotage_CircuitBreaker_NonConsecutiveFailuresDontOpen (fallos no consecutivos jamás abren) + TestSabotage_RateLimitWindow_Compacts — 2026-06-11
- [ ] **docs-001**: `docs/mcp/resilience.md` → diferido; comportamiento documentado en headers de resilience.go + este tasks.md
