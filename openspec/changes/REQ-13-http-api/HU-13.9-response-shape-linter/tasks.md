# Tasks: HU-13.9-response-shape-linter

- [ ] **rsl-001**: `cmd/domain-lint-api/`
- [ ] **rsl-002**: Validators: ListShape, SingleShape, ErrorShape, StatusCode, Header, KebabURL
- [ ] **rsl-003**: AST scan routes registered
- [ ] **rsl-004**: Snapshot file testdata/api/{error_codes,endpoint_shapes}.json
- [ ] **rsl-005**: --update mode
- [ ] **rsl-006**: Makefile `api-lint`, `api-snapshot-update`
- [ ] **rsl-007**: CI step en HU-19.1
- [ ] **test-001**: Error shape malformed → fail
- [ ] **test-002**: POST sin 201 → fail
- [ ] **test-003**: URL snake_case → fail
- [ ] **test-004**: Snapshot diff sin update → fail
- [ ] **test-005**: Update mode regenera
- [ ] **docs-001**: `docs/api/lint.md`
