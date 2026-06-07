# Tasks: HU-26.4-graceful-shutdown

- [ ] **gs-001**: Package internal/shutdown/Coordinator
- [ ] **gs-002**: Signal handler en cmd/domain-mcp/main.go
- [ ] **gs-003**: Readiness state atomic
- [ ] **gs-004**: Sequence drain implementation
- [ ] **gs-005**: Worker context cancel propagation
- [ ] **gs-006**: Pool close
- [ ] **gs-007**: Linter detecta workers sin select ctx.Done()
- [ ] **gs-008**: Métricas shutdown
- [ ] **gs-009**: Helm config terminationGracePeriodSeconds=30 + preStop hook 5s
- [ ] **test-001**: Signal orden correcto
- [ ] **test-002**: HTTP in-flight termina
- [ ] **test-003**: Worker mid-step checkpoint
- [ ] **test-004**: Timeout forced
- [ ] **test-005**: /health/ready 503 durante drain
- [ ] **docs-001**: `docs/operations/graceful-shutdown.md`
