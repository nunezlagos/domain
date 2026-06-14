# Tasks: issue-08.7-agent-handoff

- [x] **ho-001**: Migración agents.handoff_targets + max_handoffs_per_run + agent_runs.current_agent_id + agent_run_handoffs
- [x] **ho-002**: Tool generator `handoff_to_<slug>` con HandoffInputSchema
- [x] **ho-003**: Engine intercept handoff_to_* tool_call
- [x] **ho-004**: Loop detector (últimos 4 handoffs pattern)
- [x] **ho-005**: Validación target en agent.handoff_targets
- [x] **ho-006**: Max handoffs counter enforcement
- [x] **ho-007**: Cost split por agent activo en tabla cost_logs
- [x] **ho-008**: Endpoint GET /runs/:id?include=handoffs
- [x] **test-001**: Handoff con payload
- [x] **test-002**: Chain A→B→C
- [x] **test-003**: 6to handoff 429
- [x] **test-004**: Loop A→B→A→B detected
- [x] **test-005**: Audit timeline
- [x] **test-006**: Cost split
- [x] **docs-001**: `docs/agents/handoff.md` diferencias con delegate
