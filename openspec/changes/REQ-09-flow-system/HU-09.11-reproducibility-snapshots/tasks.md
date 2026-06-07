# Tasks: HU-09.11-reproducibility-snapshots

- [ ] **rs-001**: Migración flow_runs.snapshot + snapshot_s3_key + replay_*
- [ ] **rs-002**: Snapshot capturer al boot del run
- [ ] **rs-003**: ExecContext.Now / Rand inyectados
- [ ] **rs-004**: Linter test: no time.Now() ni math/rand directo en steps
- [ ] **rs-005**: LLM call interceptor con cache lookup
- [ ] **rs-006**: Endpoint POST /runs/:id/replay
- [ ] **rs-007**: Snapshot spillover S3 >1MB
- [ ] **test-001**: Snapshot capturado
- [ ] **test-002**: Replay deterministic outputs match
- [ ] **test-003**: Override applies only delta
- [ ] **test-004**: LLM cache hit no llama provider
- [ ] **test-005**: Opt-out reducido
- [ ] **sabotaje-001**: step usa time.Now() → linter falla
- [ ] **docs-001**: `docs/flows/reproducibility.md`
