# Tasks: issue-09.6-durable-execution

- [x] **de-001**: Migración flow_runs.worker_id, last_heartbeat_at, cursor, recovery_count → migration 000076
- [x] **de-002**: Migración flow_run_steps → migration 000076
- [x] **de-003**: Worker claim atómico → `claim.go` (FOR UPDATE SKIP LOCKED)
- [x] **de-004**: Heartbeat goroutine 30s → `heartbeat.go`
- [x] **de-005**: Recovery scanner 60s → `recovery.go` (LockKeyFlowRecovery)
- [x] **de-006**: Resume engine que parte desde cursor → `resume.go`
- [x] **de-007**: Output gzip + S3 spillover >10MB → `durable.go` CompressOutput/ShouldSpillToS3
- [x] **de-008**: Idempotency key auto = flow_run:step → `durable.go` StepIDempotencyKey
- [x] **de-009**: Replay_safe=false pausa awaiting_human → `durable.go` IsReplaySafe
- [x] **de-010**: Métrica `domain_flow_heartbeat_age_seconds` → metrics.go FlowHeartbeatAgeSeconds
- [x] **test-001**: Checkpoint per step → TestExtractCompletedIDs + integration
- [x] **test-002**: Resume sin re-correr completados → TestExtractCompletedIDs
- [x] **test-003**: Crash sim → reasigna → TestRecovery_ReleaseStaleRun + TestClaimRun_Stale
- [x] **test-004**: Replay-unsafe pausa → TestIsReplaySafe + TestIsStepReplaySafe_FromMap
- [x] **test-005**: Idempotency consistente → TestStepIDempotencyKey
- [x] **test-006**: S3 spillover → TestShouldSpillToS3 (threshold unit; upload real S3 en issue-23.x storage)
- [x] **test-007**: Race 2 workers claim → TestRace_TwoWorkersClaim
- [x] **docs-001**: `docs/flows/durability.md`
