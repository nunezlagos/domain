# Proposal: issue-01.7-seeders-system

## Intención

Framework Go de seeders idempotente + versionado con `go:embed` que poblará la BD con catálogos esenciales (plans, model registry, templates skills/agents/flows, policies, error codes, crons). Idea: todo dato significativo vive en BD, el binario lleva el catalog inicial.

## Scope

**Incluye:**
- Package `internal/seeds/` con interface `Seeder`
- Registry de seeders
- `go:embed` para YAMLs/MDs
- Tabla `seed_versions` para tracking
- Flag `is_user_modified` en tablas seedables para respeto a customizations
- CLI `domain seed --all|--only=X --dry-run`
- DB advisory lock para race en seed concurrente
- UPSERT con ON CONFLICT
- Dev-only seeders

**No incluye:**
- Seeders dinámicos (descargados runtime)
- Migration de seeds (las migrations no contienen INSERT data salvo enums tiny)
- Editor UI (futuro post-MVP)

## Enfoque técnico

1. Interface `Seeder { Name() string; Version() int; Run(ctx, tx, env) (Report, error) }`
2. Registry con orden topológico (plans antes que invitations, etc.)
3. `go:embed seeds/**/*.{yaml,md,go}` (compile-time)
4. Advisory lock `pg_try_advisory_lock(SEED_LOCK_ID)` para evitar race
5. Tracking en `seed_versions(name, version, applied_at, report JSONB)`
6. Per-table convention: columnas `seed_managed BOOLEAN`, `seed_version INT`, `is_user_modified BOOLEAN`, `updated_at_from_seed TIMESTAMPTZ`

## Riesgos

- Race entre pods: advisory lock + early-exit si another pod is seeding
- Override de user mod: respeto via `is_user_modified` flag
- Binary size: YAMLs pequeños, MDs no comprimidos pero <1MB total OK
- Seed bug en prod: dry-run obligatorio en deploy nuevo

## Testing

- Boot vacío → seeds ejecutan
- Boot re-run → idempotente sin diffs
- Bump version en código → reseed esa entrada
- `is_user_modified=true` → respeta
- Dev env → DevOnly ejecuta
- Prod env → DevOnly NO ejecuta
- 2 pods boot concurrente → 1 seedeé, otro skip
- Dry-run no muta
