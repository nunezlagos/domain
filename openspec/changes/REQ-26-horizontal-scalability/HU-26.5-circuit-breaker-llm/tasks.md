# Tasks: HU-26.5-circuit-breaker-llm

- [ ] **cb-001**: CircuitedLLMClient wrapper
- [ ] **cb-002**: gobreaker per (provider,model)
- [ ] **cb-003**: agents.fallback_models migration
- [ ] **cb-004**: Engine tenta fallback en error
- [ ] **cb-005**: embedding_queue migration + worker
- [ ] **cb-006**: Métricas
- [ ] **cb-007**: Alertas: CB open >5min
- [ ] **test-001**: 5 errores → OPEN
- [ ] **test-002**: Fallback model
- [ ] **test-003**: Half-open
- [ ] **test-004**: Per-provider isolation
- [ ] **test-005**: Embedding queue
- [ ] **docs-001**: `docs/llm/circuit-breaker.md`
