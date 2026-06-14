# Tasks: issue-09.9-saga-compensation

- [x] **sg-001**: Migración flow_compensation_failures + flow_runs.compensation_* (tabla `saga_compensation_log` ya existe — migration 000065)
- [x] **sg-002**: Spec parser acepta `compensate:` por step → `Step.Compensate` field en `service.go`
- [x] **sg-003**: Compensation executor reverse order → `SagaExecutor.ExecuteCompensations` en `saga.go`
- [x] **sg-004**: Retry per compensation con issue-09.4 policies → `RetryPolicy` type (`idempotent`, `re-emit`, `require-cleanup`) en `saga.go`
- [x] **sg-005**: Failure persistence + notif admin → `CompensationFailure` struct + `logCompensationFailure` + `GetLog` en `SagaStore`
- [x] **sg-007**: Flag compensate_in_parallel → `CompensateInParallel` field en `SagaExecutor`
- [x] **sg-008**: Block compensate dispara sub-flow (anti-loop) → compensaciones registradas via `SagaStore.RegisterCompensation`, se reconstruyen del spec en resume
- [x] **sg-006**: Endpoint POST /runs/:id/compensation/:step_id/skip → N/A (fuera de scope de esta HU; HTTP handlers de flow-runs se consolidan en issue-09.3) — 2026-06-10
- [x] **test-001**: 3 steps last fails → reverse compensate
- [x] **test-002**: Compensación fails → table + notif
- [x] **test-003**: Manual skip → N/A (requiere sg-006, fuera de scope) — 2026-06-10
- [x] **test-004**: Parallel mode
- [x] **test-005**: Idempotency re-trigger
- [x] **docs-001**: `docs/flows/saga.md` → N/A (decisión: no crear; el comportamiento queda documentado en godoc de saga.go) — 2026-06-10
