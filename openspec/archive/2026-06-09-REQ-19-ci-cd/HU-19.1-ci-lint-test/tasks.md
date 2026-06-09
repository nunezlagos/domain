# Tasks: HU-19.1-ci-lint-test

- [ ] **ci-001**: `.github/workflows/ci.yml` con jobs lint, unit (matrix), integration
- [ ] **ci-002**: `.golangci.yml` con linters: govet, staticcheck, errcheck, gosec, revive, gofumpt
- [ ] **ci-003**: Setup Go con cache built-in
- [ ] **ci-004**: Integration job con testcontainers + DinD service
- [ ] **ci-005**: Codecov upload + PR comment con diff coverage
- [ ] **ci-006**: Concurrency cancel-in-progress
- [ ] **ci-007**: Branch protection: required checks `lint, unit, integration`
- [ ] **test-001**: PR fixture que rompe lint → CI rojo
- [ ] **test-002**: PR fixture que rompe test → CI rojo
- [ ] **test-003**: Verificar cache hit en 2do run
- [ ] **docs-001**: `docs/contributing.md` con CI flow
