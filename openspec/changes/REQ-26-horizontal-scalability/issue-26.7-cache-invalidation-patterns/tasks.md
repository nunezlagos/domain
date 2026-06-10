# Tasks: issue-26.7-cache-invalidation-patterns

- [ ] **ci-001**: Migration SQL functions notify_cache_invalidation + create_cache_invalidation_trigger
- [ ] **ci-002**: Apply triggers a tablas cacheadas (custom_roles, platform_policies, mcp_tool_configs, plans, model_registry, agents, skills, flows)
- [ ] **ci-003**: Package internal/cache/distributed/
- [ ] **ci-004**: Listener con session-conn dedicada + reconnect + flush
- [ ] **ci-005**: Dedupe window
- [ ] **ci-006**: Métricas
- [ ] **ci-007**: Aplicar a caches existentes (issue-02.8, issue-01.8, issue-12.6)
- [ ] **test-001**: Update → NOTIFY recibido
- [ ] **test-002**: Multi-pod consistency
- [ ] **test-003**: Reconnect flush
- [ ] **test-004**: Dedupe
- [ ] **docs-001**: `docs/architecture/cache-invalidation.md`
