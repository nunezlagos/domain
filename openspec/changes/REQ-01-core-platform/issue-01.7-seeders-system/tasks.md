# Tasks: issue-01.7-seeders-system

## Framework

- [x] **sd-001**: Package `internal/seeds/` con interface + registry
- [x] **sd-002**: go:embed catalogs dir
- [x] **sd-003**: Advisory lock orchestrator
- [x] **sd-004**: Schema seed_versions
- [x] **sd-005**: Migration agrega columns seed_* a tablas seedables (agents, skills, flows, plans, model_registry, notification_templates, platform_policies, system_crons)
- [x] **sd-006**: UPSERT helper respetando is_user_modified
- [x] **sd-007**: Report types + structured logging

## Catalogs

- [x] **sd-010**: plans.go (Free/Pro/Enterprise con limits)
- [x] **sd-011**: models.go + YAML model_registry (OpenAI, Anthropic, Google, Ollama models + pricing)
- [x] **sd-012**: agents/*.yaml (5 templates issue-08.5)
- [x] **sd-013**: skills/*.yaml (catalog inicial ~10 skills built-in)
- [x] **sd-014**: flows/*.yaml (ejemplos starter)
- [x] **sd-015**: notifications/*.yaml (otp_email, invitation_email, usage_alert, slow_query_alert)
- [x] **sd-016**: policies/*.md (copia de .claude/rules/)
- [x] **sd-017**: error_codes.yaml
- [x] **sd-018**: crons.go (system crons: backup, drift, slow-query, password rotation, schema-drift, expire-otp, expire-trash, etc.)

## CLI

- [x] **sd-020**: `domain seed --all`
- [x] **sd-021**: `--only` filtering
- [x] **sd-022**: `--dry-run` mode
- [x] **sd-023**: `--force` ignore version

## Per-env

- [x] **sd-030**: DevOnly seeder (acme-demo org, alice/bob users, fixtures)
- [x] **sd-031**: Skip DevOnly en env=prod

## Tests

- [x] **sd-040**: Empty DB boot → seeds applied
- [x] **sd-041**: Re-boot idempotent
- [x] **sd-042**: Bump version re-seed
- [x] **sd-043**: is_user_modified preserved
- [x] **sd-044**: Advisory lock race
- [x] **sd-045**: Dry-run no muta
- [x] **sd-046**: YAML inválido fail-fast
- [x] **sd-047**: Sabotaje rename slug → error

## Docs

- [x] **sd-050**: `docs/seeds.md` con cómo agregar nuevo seed + bumping version
