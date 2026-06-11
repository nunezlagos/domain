# Tasks: issue-09.8-external-signals

- [x] **sig-001**: Migración tables pending + delivered
- [ ] **sig-002**: Step type await_signal en issue-09.2 engine — runner/flow layer
- [ ] **sig-003**: Endpoint POST /runs/:id/signals — API layer
- [x] **sig-004**: Broadcast via BroadcastSignal (service layer)
- [ ] **sig-005**: LISTEN/NOTIFY wake mechanism — infra layer
- [x] **sig-006**: Early signal window persistence (ExpectSignal)
- [x] **sig-007**: Timeout integra retry policy issue-09.4 (WaitForSignal)
- [x] **test-001**: Happy await + signal
- [x] **test-002**: Timeout retry
- [x] **test-003**: Broadcast N runs
- [x] **test-004**: Sin pending 409
- [x] **test-005**: Early signal delivered later
- [ ] **docs-001**: `docs/flows/signals.md`
