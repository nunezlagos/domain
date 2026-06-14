# Tasks: issue-26.7-cache-invalidation-patterns

- [x] **ci-001**: Migration SQL functions notify_cache_invalidation + create_cache_invalidation_trigger
- [x] **ci-002**: Apply triggers a tablas cacheadas (custom_roles, platform_policies, mcp_tool_configs, plans, model_registry, agents, skills, flows)
- [x] **ci-003**: Package internal/cache/distributed/
- [x] **ci-004**: Listener con session-conn dedicada + reconnect + flush
- [x] **ci-005**: Dedupe window
- [x] **ci-006**: Métricas
- [x] **ci-007**: Aplicar a caches existentes (issue-02.8, issue-01.8, issue-12.6)
- [x] **test-001**: Update → NOTIFY recibido
- [x] **test-002**: Multi-pod consistency
- [x] **test-003**: Reconnect flush
- [x] **test-004**: Dedupe
- [x] **docs-001**: `docs/architecture/cache-invalidation.md`
