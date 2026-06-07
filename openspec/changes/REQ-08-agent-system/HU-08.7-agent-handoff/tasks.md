# Tasks: HU-08.7-agent-handoff

- [ ] **ho-001**: Migración agents.handoff_targets + max_handoffs_per_run + agent_runs.current_agent_id + agent_run_handoffs
- [ ] **ho-002**: Tool generator `handoff_to_<slug>` con HandoffInputSchema
- [ ] **ho-003**: Engine intercept handoff_to_* tool_call
- [ ] **ho-004**: Loop detector (últimos 4 handoffs pattern)
- [ ] **ho-005**: Validación target en agent.handoff_targets
- [ ] **ho-006**: Max handoffs counter enforcement
- [ ] **ho-007**: Cost split por agent activo en tabla cost_logs
- [ ] **ho-008**: Endpoint GET /runs/:id?include=handoffs
- [ ] **test-001**: Handoff con payload
- [ ] **test-002**: Chain A→B→C
- [ ] **test-003**: 6to handoff 429
- [ ] **test-004**: Loop A→B→A→B detected
- [ ] **test-005**: Audit timeline
- [ ] **test-006**: Cost split
- [ ] **docs-001**: `docs/agents/handoff.md` diferencias con delegate
