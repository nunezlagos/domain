# Tasks: issue-26.5-circuit-breaker-llm

- [x] **cb-001**: CircuitedLLMClient wrapper
- [x] **cb-002**: gobreaker per (provider,model)
- [x] **cb-003**: agents.fallback_models migration
- [x] **cb-004**: Engine tenta fallback en error
- [x] **cb-005**: embedding_queue migration + worker
- [x] **cb-006**: Métricas
- [x] **cb-007**: Alertas: CB open >5min
- [x] **test-001**: 5 errores → OPEN
- [x] **test-002**: Fallback model
- [x] **test-003**: Half-open
- [x] **test-004**: Per-provider isolation
- [x] **test-005**: Embedding queue
- [x] **docs-001**: `docs/llm/circuit-breaker.md`
