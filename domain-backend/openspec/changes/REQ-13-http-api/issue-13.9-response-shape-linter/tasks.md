# Tasks: issue-13.9-response-shape-linter

- [x] **rsl-001**: CLI → `cmd/response-shape-lint` (nombre definitivo; el plan original decía cmd/domain-lint-api)
- [x] **rsl-002**: Validators → shape de writers (writeData/writeError obligatorios), KebabURL, POST create→201; ListShape/SingleShape/ErrorShape enforced por construcción al prohibir writes crudos — 2026-06-10
- [x] **rsl-003**: AST scan routes registered → extractRoutes sobre mux.HandleFunc en api.go — 2026-06-10
- [x] **rsl-004**: Snapshot testdata → internal/api/handler/testdata/api/{endpoint_shapes,error_codes}.json — 2026-06-10
- [x] **rsl-005**: --update mode → flag -update regenera ambos snapshots — 2026-06-10
- [x] **rsl-006**: Makefile `api-lint`, `api-snapshot-update` — 2026-06-10
- [x] **rsl-007**: CI step → job response-shape-lint en ci.yml — 2026-06-10
- [x] **test-001**: Error shape malformed → fail → fixtures bad_* (raw Write/Encoder/Fprintf/WriteHeader) + TestRealAPI_HandlersUseCanonicalWriters
- [x] **test-002**: POST sin 201 → fail → TestRoutes_PostCreateWithout201_Fails — 2026-06-10
- [x] **test-003**: URL snake_case → fail → TestRoutes_SnakeCaseURL_Fails — 2026-06-10
- [x] **test-004**: Snapshot diff sin update → fail → TestSnapshot_DriftWithoutUpdate_Fails — 2026-06-10
- [x] **test-005**: Update mode regenera → TestSnapshot_UpdateRegenerates — 2026-06-10
- [x] **docs-001**: `docs/api/lint.md` — 2026-06-10
