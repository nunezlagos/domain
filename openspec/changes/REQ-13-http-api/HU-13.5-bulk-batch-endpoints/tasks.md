# Tasks: HU-13.5-bulk-batch-endpoints

- [ ] **bb-001**: Handler POST /observations/batch
- [ ] **bb-002**: Handler POST /knowledge_docs/batch
- [ ] **bb-003**: Handler POST /prompts/batch
- [ ] **bb-004**: Handler DELETE /observations/batch
- [ ] **bb-005**: Per-item validator + aggregator errors
- [ ] **bb-006**: all_or_nothing con pgx.CopyFrom
- [ ] **bb-007**: best_effort per-item tx
- [ ] **bb-008**: Streaming JSON parser
- [ ] **bb-009**: Idempotency compatibility
- [ ] **bb-010**: Batch event publication (no N+1)
- [ ] **test-001**: 500 items happy
- [ ] **test-002**: all_or_nothing rollback
- [ ] **test-003**: best_effort partial
- [ ] **test-004**: 5001 → 413
- [ ] **test-005**: Bulk delete RBAC mixto
- [ ] **docs-001**: `docs/api/batch.md`
