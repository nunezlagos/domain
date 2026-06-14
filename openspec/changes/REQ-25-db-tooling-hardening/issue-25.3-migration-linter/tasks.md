# Tasks: issue-25.3-migration-linter

- [x] **ml-001**: `.squawk.toml` con reglas (complementario opcional; enforcement primario es db-conventions-lint) — 2026-06-10
- [x] **ml-002**: CI step → job `db-conventions-lint` en .github/workflows/ci.yml (build + run sobre migrations)
- [x] **ml-003**: Makefile target `db-lint` (+ `db-lint-fix`) — 2026-06-10
- [x] **ml-004**: Header convention linter (Go) → internal/dbconvlint checkHeader (6 campos)
- [x] **ml-005**: Pre-commit hook opcional → scripts/githooks/pre-commit + `make install-githooks` — 2026-06-10
- [x] **ml-006**: Docs reglas + cómo overridear → docs/db/migrations.md — 2026-06-10
- [x] **test-001**: CREATE INDEX sin CONCURRENTLY → fail → TestSafety_CreateIndexConcurrent
- [x] **test-002**: Override comment pasa → TestSafety_Override + TestLint_OverrideNextLine
- [x] **test-003**: NOT NULL sin DEFAULT → fail → TestSafety_AddColumnNotNullSinDefault
- [x] **test-004**: Clean migration → pass → casos `_OK` (CreateIndexConcurrent_OK, NotNullConDefault_OK, DropTableConIfExists_OK, FKConNotValid_OK)
- [x] **test-005**: Header missing warning → TestLint_HeaderMissing + TestLint_DownMigrationNoHeader
- [x] **docs-001**: `docs/db/migrations.md` con conventions + override — 2026-06-10
