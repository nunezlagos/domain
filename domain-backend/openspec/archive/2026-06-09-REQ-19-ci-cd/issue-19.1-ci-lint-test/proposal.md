# Proposal: issue-19.1-ci-lint-test

## Intención

CI pipeline GitHub Actions con lint + unit + integration + coverage en cada PR/push, con caching agresivo y branch protection que bloquea merge si rojo.

## Scope

**Incluye:**
- `.github/workflows/ci.yml` con jobs lint, unit, integration, coverage
- golangci-lint config `.golangci.yml` con linters: govet, staticcheck, errcheck, gosec, revive, gofumpt
- Setup Go con cache y `actions/cache@v4` para `~/go/pkg/mod` + `~/.cache/go-build`
- Job integration con docker-in-docker para testcontainers (Postgres + MinIO)
- Codecov upload o gocovsh comment
- Branch protection rules definidas en `.github/branch-protection.yml` (settings reproducible)

**No incluye:**
- E2E tests (otra HU si aplica)
- Security scanning (Snyk/Trivy/Dependabot ya en GitHub)

## Enfoque técnico

1. Matrix Go versions [1.22, 1.23] para detectar incompatibilidades
2. `go test -race -coverprofile=coverage.out -covermode=atomic ./...`
3. testcontainers usa Docker layer caching para imágenes pgvector/minio
4. Coverage gate: PR no puede bajar coverage del baseline >2%

## Riesgos

- Flakiness integration: usar retry-go on test layer no en CI level
- Minutes Actions: cache + concurrency limits + cancel-in-progress
- Docker-in-docker permissions: usar `services` o Docker socket mount

## Testing

- PR ficticio que rompe lint → CI rojo en <3 min
- PR ficticio que rompe test → CI rojo
- PR limpio merge: green checks en <8 min total
