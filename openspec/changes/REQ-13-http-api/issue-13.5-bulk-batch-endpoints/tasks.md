# Tasks: issue-13.5-bulk-batch-endpoints

- [x] **bb-001**: Handler POST /observations/batch
- [x] **bb-002**: Handler POST /knowledge_docs/batch
- [x] **bb-003**: Handler POST /prompts/batch
- [x] **bb-004**: Handler DELETE /observations/batch
- [x] **bb-005**: Per-item validator + aggregator errors
- [x] **bb-006**: all_or_nothing con pgx.CopyFrom
- [x] **bb-007**: best_effort per-item tx
- [x] **bb-008**: Streaming JSON parser
- [x] **bb-009**: Idempotency compatibility
- [x] **bb-010**: Batch event publication (no N+1)
- [x] **test-001**: 500 items happy
- [x] **test-002**: all_or_nothing rollback
- [x] **test-003**: best_effort partial
- [x] **test-004**: 5001 → 413
- [x] **test-005**: Bulk delete RBAC mixto
- [x] **docs-001**: `docs/api/batch.md`
