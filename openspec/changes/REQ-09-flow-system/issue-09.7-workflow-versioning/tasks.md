# Tasks: issue-09.7-workflow-versioning

- [x] **fv-001**: Migración flow_versions + flow_runs.flow_version_id
- [x] **fv-002**: PATCH /flows/:id crea draft (no muta current) → handler/flow.go updateFlow (NewVersion draft + 422 spec inválido) — 2026-06-10
- [x] **fv-003**: POST publish con tx is_default flip
- [x] **fv-004**: POST deprecate
- [x] **fv-005**: GET versions list
- [x] **fv-006**: GET diff estructural (Spec-based comparison)
- [x] **fv-007**: Breaking change detector
- [x] **fv-008**: Engine usa flow_version_id del run → versionpin.go (pin idempotente por hash en Run + RunInput.FlowVersion published-only + resume lee versión pinneada) — 2026-06-10
- [x] **fv-009**: Cron archive deprecated >90d sin runs → VersioningStore.ArchiveDeprecated + runFlowVersionArchiver diario en leader (cmd/domain) — 2026-06-10
- [x] **test-001**: Draft no es invocable
- [x] **test-002**: Publish flipea default
- [x] **test-003**: Run en vuelo conserva versión
- [x] **test-004**: Invoke versión específica
- [x] **test-005**: Deprecate 410 nuevos
- [x] **test-006**: Diff estructural
- [x] **test-007**: Breaking flags
- [x] **docs-001**: `docs/flows/versioning.md` — 2026-06-10
