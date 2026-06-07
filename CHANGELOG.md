# Changelog

Todos los cambios notables a este proyecto se documentan en este archivo.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/) y este proyecto adhiere a [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Commits siguen [Conventional Commits](https://www.conventionalcommits.org/) según `.claude/rules/git.md`.

## [Unreleased]

### Added
- Spec inicial: 27 REQs, 148 HUs en `openspec/changes/`
- 5 RFCs de boundaries arquitectónicas en `docs/rfc/`
- 9 reglas de conventions en `.claude/rules/`
- Roadmap detallado con 6 fases en `docs/roadmap.md`
- Sistema de policies en BD (HU-01.8) — DB como source of truth
- Seeders Go embebidos (HU-01.7) para catálogos iniciales
- MCP tool resilience strict (HU-12.6) con timeout + CB + cache LRU
- DB tooling + hardening (REQ-25, 13 HUs): PgBouncer, RLS, pgaudit, read replicas, password rotation, anonymization, etc.
- Horizontal scalability patterns (REQ-26, 7 HUs)
- Vertical performance tuning (REQ-27, 4 HUs)

### Changed
- HU-02.7 reescrita de Google OAuth a passwordless OTP por email con RUT/email identifier

### Notes
- Status del backlog: 100% `proposed`, 0 HUs implementadas
- Próximo paso: kickoff Fase 0 (bootstrap dev environment) según `docs/roadmap.md`

---

## Plantilla para futuros releases

```markdown
## [vX.Y.Z] - YYYY-MM-DD

### Added
- Nueva feature backwards-compatible (commits `feat:`)

### Changed
- Cambio en comportamiento existente (commits `refactor:` con impacto visible)

### Deprecated
- Features que se removerán en próximos releases

### Removed
- Features eliminadas en este release (commits `feat!:` con removal)

### Fixed
- Bug fixes (commits `fix:`)

### Security
- Patches de seguridad
```

[Unreleased]: https://github.com/saargo/domain/compare/v0.0.0...HEAD
