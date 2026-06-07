# Tasks: HU-09.9-saga-compensation

- [ ] **sg-001**: Migración flow_compensation_failures + flow_runs.compensation_*
- [ ] **sg-002**: Spec parser acepta `compensate:` por step
- [ ] **sg-003**: Compensation executor reverse order
- [ ] **sg-004**: Retry per compensation con HU-09.4 policies
- [ ] **sg-005**: Failure persistence + notif admin
- [ ] **sg-006**: Endpoint POST /runs/:id/compensation/:step_id/skip
- [ ] **sg-007**: Flag compensate_in_parallel
- [ ] **sg-008**: Block compensate dispara sub-flow (anti-loop)
- [ ] **test-001**: 3 steps last fails → reverse compensate
- [ ] **test-002**: Compensación fails → table + notif
- [ ] **test-003**: Manual skip
- [ ] **test-004**: Parallel mode
- [ ] **test-005**: Idempotency re-trigger
- [ ] **docs-001**: `docs/flows/saga.md`
