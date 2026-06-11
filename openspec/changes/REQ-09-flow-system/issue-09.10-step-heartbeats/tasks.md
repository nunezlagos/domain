# Tasks: issue-09.10-step-heartbeats

- [x] **hb-001**: Migración flow_run_steps progress + heartbeat
- [x] **hb-002**: ExecContext.Heartbeat con throttle 5s → stepheartbeat.go StepHeartbeater (WithHeartbeater/HeartbeaterFrom + RunInput.Heartbeat en steptypes; beginStepRow/completeStepRow en loop) — 2026-06-10
- [x] **hb-003**: Zombie detector cron (Watchdog + FindStuck + FindStuckWithCustomThreshold)
- [x] **hb-004**: SSE event publisher via NOTIFY → heartbeats.go NotifyProgress (pg_notify flow_step_progress + ProgressEvent JSON); consumidor SSE en issue-09.3 — 2026-06-10
- [x] **hb-005**: heartbeat_threshold_seconds por step type defaults (FindStuckWithCustomThreshold)
- [x] **test-001**: Heartbeat actualiza (BeatWithProgress)
- [x] **test-002**: Throttle batch
- [x] **test-003**: Zombie tras threshold
- [x] **test-004**: SSE evento
- [x] **test-005**: Short step exempt
- [x] **docs-001**: `docs/flows/heartbeats.md` — 2026-06-10
