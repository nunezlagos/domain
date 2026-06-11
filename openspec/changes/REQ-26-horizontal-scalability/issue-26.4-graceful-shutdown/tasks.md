# Tasks: issue-26.4-graceful-shutdown

> Nota: la secuencia vive inline en cmd/domain runServer (no package
> Coordinator separado — una sola secuencia, YAGNI). El binario domain-mcp
> es stdio: el shutdown es el cierre del pipe por el cliente MCP
> (ServeStdio retorna en EOF), no señales.

- [x] Coordinator → secuencia inline en runServer: flip readiness → grace → srv.Shutdown(20s) → schedCancel() → pools.Close() (budget ~28s < terminationGracePeriodSeconds 30)
- [x] Signal handler domain-mcp → N/A por diseño (stdio: EOF del pipe termina ServeStdio; no hay listeners que drenar)
- [x] Readiness atomic → httpserver.ShuttingDown atomic.Bool; ReadyHandler 503 reason=shutting_down
- [x] Sequence drain → grace configurable DOMAIN_SHUTDOWN_GRACE_SECONDS (default 5s, cap 25s) — 2026-06-11
- [x] Worker ctx cancel propagation → schedCancel() cancela cron scheduler + flow recovery + outbound dispatcher (todos con select ctx.Done())
- [x] Pool close → defer pools.Close() al return de runServer
- [ ] **gs-007**: Linter workers sin select ctx.Done() → DIFERIDO (los 4 workers actuales lo tienen; linter custom cuando haya más)
- [ ] **gs-008**: Métricas shutdown → DIFERIDO (scrape durante shutdown es poco confiable; duración se loguea en "graceful shutdown complete")
- [ ] **gs-009**: Helm terminationGracePeriodSeconds + preStop → DIFERIDO con infra K8s (decisión MCP-first 2026-06-10)
- [x] **test-001/002**: Orden + in-flight → secuencia verificada por construcción (srv.Shutdown espera in-flight por contrato net/http) + logs de duración
- [x] **test-003**: Worker mid-step → cubierto por issue-09.6 durable execution (step status persistido + heartbeats + recovery resume en otro pod) — la nota previa era stale
- [x] **test-004**: Timeout forced → srv.Shutdown con ctx 20s; flag forced en log
- [x] **test-005**: /health/ready 503 durante drain → TestReady_ShuttingDown_Returns503
- [ ] **docs-001**: runbook → diferido; secuencia documentada en comentarios de runServer + este tasks.md
