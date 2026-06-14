# Tasks: issue-09.8-external-signals

- [x] **sig-001**: Migración tables pending + delivered
- [x] **sig-002**: Step type await_signal en engine → runner.go execWaitSignal (paused_awaiting_signal + ExpectSignal + WaitNotify + output payload) — 2026-06-10
- [x] **sig-003**: Endpoint POST /runs/:id/signals → handler/flow.go signalFlowRun (202/404/409/422 + audit flow.signal_delivered) — 2026-06-10
- [x] **sig-004**: Broadcast via BroadcastSignal (service layer)
- [x] **sig-005**: LISTEN/NOTIFY wake mechanism → signals.go Send/Broadcast emiten pg_notify(flow_signals); WaitNotify bloquea en WaitForNotification con fallback polling — 2026-06-10
- [x] **sig-006**: Early signal window persistence (ExpectSignal)
- [x] **sig-007**: Timeout integra retry policy issue-09.4 (WaitForSignal)
- [x] **test-001**: Happy await + signal
- [x] **test-002**: Timeout retry
- [x] **test-003**: Broadcast N runs
- [x] **test-004**: Sin pending 409
- [x] **test-005**: Early signal delivered later
- [x] **docs-001**: `docs/flows/signals.md` — 2026-06-10
