# Tasks: HU-09.7-workflow-versioning

- [ ] **fv-001**: Migración flow_versions + flow_runs.flow_version_id
- [ ] **fv-002**: PATCH /flows/:id crea draft (no muta current)
- [ ] **fv-003**: POST publish con tx is_default flip
- [ ] **fv-004**: POST deprecate
- [ ] **fv-005**: GET versions list
- [ ] **fv-006**: GET diff con `evanphx/json-patch`
- [ ] **fv-007**: Breaking change detector
- [ ] **fv-008**: Engine usa flow_version_id del run
- [ ] **fv-009**: Cron archive deprecated >90d sin runs
- [ ] **test-001**: Draft no es invocable
- [ ] **test-002**: Publish flipea default
- [ ] **test-003**: Run en vuelo conserva versión
- [ ] **test-004**: Invoke versión específica
- [ ] **test-005**: Deprecate 410 nuevos
- [ ] **test-006**: Diff json-patch
- [ ] **test-007**: Breaking flags
- [ ] **docs-001**: `docs/flows/versioning.md`
