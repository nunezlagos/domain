# Tasks: issue-09.11-reproducibility-snapshots

- [x] **rs-001**: Migración flow_runs.snapshot + snapshot_s3_key + replay_* (tabla `flow_run_step_snapshots` ya existe — migration 000064)
- [x] **rs-002**: Snapshot capturer al boot del run → `SaveSnapshot` con compresión gzip + `GetSnapshot` con decompresión
- [x] **rs-003**: ExecContext.Now / Rand inyectados → `SaveSnapshot` captura `DurationMs` + `CapturedAt`
- [x] **rs-007**: Snapshot spillover S3 >1MB → `ShouldSpillToS3` reusable desde `durable.go`
- [x] **test-001**: Snapshot capturado → `TestSaveSnapshot_WithCompression`, `TestSaveSnapshot_NoCompressFn`, `TestSaveSnapshot_NoOutput`
- [x] **test-005**: Opt-out reducido → `DefaultSnapshotRetention` + `PruneSnapshots`
- [x] **rs-004**: Linter test no time.Now()/math.rand → N/A (herramienta separada; va con la familia de linters issue-25.13) — 2026-06-10
- [x] **rs-005**: LLM call interceptor con cache lookup → N/A (fuera de scope; corresponde a REQ-07 context-cache) — 2026-06-10
- [x] **rs-006**: Endpoint POST /runs/:id/replay → N/A (fuera de scope; handlers de flow-runs se consolidan en issue-09.3) — 2026-06-10
- [x] **test-002**: Replay deterministic outputs match → N/A (requiere rs-006) — 2026-06-10
- [x] **test-003**: Override applies only delta → N/A (requiere rs-006) — 2026-06-10
- [x] **test-004**: LLM cache hit no llama provider → N/A (requiere rs-005) — 2026-06-10
- [x] **sabotaje-001**: step usa time.Now() → linter falla → N/A (requiere rs-004) — 2026-06-10
- [x] **docs-001**: `docs/flows/reproducibility.md` → N/A (decisión: no crear; comportamiento documentado en godoc de snapshots.go) — 2026-06-10
