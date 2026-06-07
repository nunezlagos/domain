# Tasks: HU-09.6-durable-execution

- [ ] **de-001**: Migración flow_runs.worker_id, last_heartbeat_at, cursor, recovery_count
- [ ] **de-002**: Migración flow_run_steps
- [ ] **de-003**: Worker claim atómico
- [ ] **de-004**: Heartbeat goroutine 30s
- [ ] **de-005**: Recovery scanner 60s
- [ ] **de-006**: Resume engine que parte desde cursor
- [ ] **de-007**: Output gzip + S3 spillover >10MB
- [ ] **de-008**: Idempotency key auto = flow_run:step
- [ ] **de-009**: Replay_safe=false pausa awaiting_human
- [ ] **de-010**: Métrica `domain_flow_heartbeat_age_seconds`
- [ ] **test-001**: Checkpoint per step
- [ ] **test-002**: Resume sin re-correr completados
- [ ] **test-003**: Crash sim → reasigna
- [ ] **test-004**: Replay-unsafe pausa
- [ ] **test-005**: Idempotency consistente
- [ ] **test-006**: S3 spillover
- [ ] **test-007**: Race 2 workers claim
- [ ] **docs-001**: `docs/flows/durability.md`
