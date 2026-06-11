# Tasks: issue-09.7-workflow-versioning

- [x] **fv-001**: Migración flow_versions + flow_runs.flow_version_id
- [ ] **fv-002**: PATCH /flows/:id crea draft (no muta current) — API layer
- [x] **fv-003**: POST publish con tx is_default flip
- [x] **fv-004**: POST deprecate
- [x] **fv-005**: GET versions list
- [x] **fv-006**: GET diff estructural (Spec-based comparison)
- [x] **fv-007**: Breaking change detector
- [ ] **fv-008**: Engine usa flow_version_id del run — runner/flow layer
- [ ] **fv-009**: Cron archive deprecated >90d sin runs — infra layer
- [x] **test-001**: Draft no es invocable
- [x] **test-002**: Publish flipea default
- [x] **test-003**: Run en vuelo conserva versión
- [x] **test-004**: Invoke versión específica
- [x] **test-005**: Deprecate 410 nuevos
- [x] **test-006**: Diff estructural
- [x] **test-007**: Breaking flags
- [ ] **docs-001**: `docs/flows/versioning.md`
