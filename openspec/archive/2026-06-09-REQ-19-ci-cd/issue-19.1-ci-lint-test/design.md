# Design: issue-19.1-ci-lint-test

## Decisión arquitectónica

**CI:** GitHub Actions (mismo proveedor que el repo).
**Linter:** golangci-lint v1.60 con preset estricto.
**Test runner:** `go test -race` con coverage atomic.
**Integration:** testcontainers (no Docker compose en CI).
**Cache:** `actions/setup-go@v5` (built-in) + custom `actions/cache@v4` para go-build.

## Alternativas descartadas

- **CircleCI / GitLab CI:** sin razón para fragmentar tooling
- **Docker compose en CI:** menos paralelismo que testcontainers
- **Sin matrix versions:** perdemos detección temprana de breakage en upgrades Go

## Workflow

```yaml
name: ci
on: { pull_request: {}, push: { branches: [main] } }
concurrency: { group: ci-${{ github.ref }}, cancel-in-progress: true }
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23', cache: true }
      - uses: golangci/golangci-lint-action@v6
        with: { version: v1.60 }
  unit:
    runs-on: ubuntu-latest
    strategy: { matrix: { go: ['1.22', '1.23'] } }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: ${{ matrix.go }}, cache: true }
      - run: go test -race -coverprofile=coverage.out ./...
      - uses: codecov/codecov-action@v4
  integration:
    runs-on: ubuntu-latest
    services:
      docker: { image: docker:dind }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23', cache: true }
      - run: go test -tags=integration -timeout=10m ./...
```

## TDD plan

1. PR rompiendo `go vet` → lint job rojo
2. PR rompiendo test unit → unit job rojo
3. PR limpio → todos verdes en <8 min
4. Cache hit en 2do run (verificable en logs Actions)
