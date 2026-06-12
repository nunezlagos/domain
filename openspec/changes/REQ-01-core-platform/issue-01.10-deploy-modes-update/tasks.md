# Tasks: issue-01.10-deploy-modes-update

- [ ] **inst-001**: HU spec completo (issue.md, design.md, proposal.md, tasks.md, state.yaml) — 2026-06-11
- [ ] **inst-002**: internal/cli/onboard/deployment.go: selector de mode + docker compose / DSN / hybrid
- [ ] **inst-003**: internal/cli/onboard/backup.go: helpers para backup + restore con timestamp RFC3339
- [ ] **inst-004**: cmd/domain/main.go: nuevo subcommand `install` (alias de onboard con superpoderes)
- [ ] **inst-005**: wizard.go: detección de estado actual (installed / fresh / partial) + flujo install
- [ ] **inst-006**: wizard.go: AGENTS.md injection con marker `<!-- domain-managed -->`
- [ ] **inst-007**: cmd/domain/main.go: nuevo subcommand `update` (backups + migrate + seed + init --no-stub)
- [ ] **inst-008**: cmd/domain/main.go: nuevo subcommand `restore` (one-shot: restaura 1 archivo)
- [ ] **inst-009**: internal/seeds/seeds.go: exponer `domain seed all` que corre todos los seeders
- [ ] **inst-010**: service.go (workflowimport): agregar `UpdateOnly` mode (skip disco, solo verifica BD)
- [ ] **inst-011**: tests: deployment modes (local/cloud/hybrid), DSN validation, backup/restore, idempotency, AGENTS.md
- [ ] **inst-012**: tests sabotaje: race condition (2 installs), DSN sin sslmode, AGENTS.md invasion
- [ ] **inst-013**: integration test E2E con docker compose (levanta stack, corre install, valida)
- [ ] **inst-014**: docs/GETTING_STARTED.md: nueva seccion sobre install vs update vs restore
- [ ] **inst-015**: state.yaml → implemented + commit final
