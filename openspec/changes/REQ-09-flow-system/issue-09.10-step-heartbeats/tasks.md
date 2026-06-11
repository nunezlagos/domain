# Tasks: issue-09.10-step-heartbeats

- [x] **hb-001**: Migración flow_run_steps progress + heartbeat
- [ ] **hb-002**: ExecContext.Heartbeat con throttle 5s — runner/flow layer
- [x] **hb-003**: Zombie detector cron (Watchdog + FindStuck + FindStuckWithCustomThreshold)
- [ ] **hb-004**: SSE event publisher via NOTIFY — infra layer
- [x] **hb-005**: heartbeat_threshold_seconds por step type defaults (FindStuckWithCustomThreshold)
- [x] **test-001**: Heartbeat actualiza (BeatWithProgress)
- [x] **test-002**: Throttle batch
- [x] **test-003**: Zombie tras threshold
- [x] **test-004**: SSE evento
- [x] **test-005**: Short step exempt
- [ ] **docs-001**: `docs/flows/heartbeats.md`
