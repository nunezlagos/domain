# Tasks: HU-01.7-seeders-system

## Framework

- [ ] **sd-001**: Package `internal/seeds/` con interface + registry
- [ ] **sd-002**: go:embed catalogs dir
- [ ] **sd-003**: Advisory lock orchestrator
- [ ] **sd-004**: Schema seed_versions
- [ ] **sd-005**: Migration agrega columns seed_* a tablas seedables (agents, skills, flows, plans, model_registry, notification_templates, platform_policies, system_crons)
- [ ] **sd-006**: UPSERT helper respetando is_user_modified
- [ ] **sd-007**: Report types + structured logging

## Catalogs

- [ ] **sd-010**: plans.go (Free/Pro/Enterprise con limits)
- [ ] **sd-011**: models.go + YAML model_registry (OpenAI, Anthropic, Google, Ollama models + pricing)
- [ ] **sd-012**: agents/*.yaml (5 templates HU-08.5)
- [ ] **sd-013**: skills/*.yaml (catalog inicial ~10 skills built-in)
- [ ] **sd-014**: flows/*.yaml (ejemplos starter)
- [ ] **sd-015**: notifications/*.yaml (otp_email, invitation_email, usage_alert, slow_query_alert)
- [ ] **sd-016**: policies/*.md (copia de .claude/rules/)
- [ ] **sd-017**: error_codes.yaml
- [ ] **sd-018**: crons.go (system crons: backup, drift, slow-query, password rotation, schema-drift, expire-otp, expire-trash, etc.)

## CLI

- [ ] **sd-020**: `domain seed --all`
- [ ] **sd-021**: `--only` filtering
- [ ] **sd-022**: `--dry-run` mode
- [ ] **sd-023**: `--force` ignore version

## Per-env

- [ ] **sd-030**: DevOnly seeder (acme-demo org, alice/bob users, fixtures)
- [ ] **sd-031**: Skip DevOnly en env=prod

## Tests

- [ ] **sd-040**: Empty DB boot → seeds applied
- [ ] **sd-041**: Re-boot idempotent
- [ ] **sd-042**: Bump version re-seed
- [ ] **sd-043**: is_user_modified preserved
- [ ] **sd-044**: Advisory lock race
- [ ] **sd-045**: Dry-run no muta
- [ ] **sd-046**: YAML inválido fail-fast
- [ ] **sd-047**: Sabotaje rename slug → error

## Docs

- [ ] **sd-050**: `docs/seeds.md` con cómo agregar nuevo seed + bumping version
