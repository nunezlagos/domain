# Git & Versioning Conventions — Domain

## Repo es LOCAL-ONLY hasta orden explícita

**Estado actual: NO hay remote. Branch `main` solo en `.git/` local.**

Reglas inmediatas (vigentes hasta que el usuario diga lo contrario):

- ❌ **NO ejecutar** `git push`, `git push --set-upstream`, ni equivalentes
- ❌ **NO crear remotes** con `git remote add`
- ❌ **NO crear repos** GitHub/GitLab/Bitbucket con `gh repo create` u equivalentes
- ❌ **NO subir tags** con `git push --tags`
- ✅ Sí permitido: `git commit`, `git log`, `git status`, `git diff`, branches locales, tags locales

El agente NO debe sugerir push/remote en cada commit. Cuando el usuario decida abrir
remote, dará instrucción explícita literal (ej. "crea repo en github" o "haz push").
Hasta entonces, asumir local-only es el estado correcto y no preguntar repetidamente.

Si una herramienta o flujo necesita remote para funcionar (ej. webhook delivery
con repo público), notificar al usuario el bloqueo y esperar respuesta.

## Branching

- `main` — única branch protegida; siempre deployable
- feature branches: `feat/<short-name>`, `fix/<short-name>`, `chore/<short-name>`, `docs/<short-name>`
- NO long-lived branches (`develop`, `staging`, etc.); usar tags + envs
- Branch protection en main: required checks (CI verde, 1+ review), no force-push, no delete

## Conventional Commits — obligatorio

Format: `<type>(<scope>)?: <description>`

### Types soportados

| type | uso | bumps SemVer |
|------|-----|--------------|
| `feat` | nueva feature user-facing | minor |
| `fix` | bug fix | patch |
| `perf` | mejora de performance | patch |
| `refactor` | refactor sin cambio funcional | patch |
| `docs` | solo cambios doc | sin bump |
| `test` | agregar/ajustar tests | sin bump |
| `build` | build system, deps | sin bump |
| `ci` | CI/CD config | sin bump |
| `chore` | tareas mantenimiento | sin bump |
| `style` | formato (no afecta lógica) | sin bump |
| `revert` | revert commit previo | depende |

### Breaking changes

Format: `feat!: ...` o body con `BREAKING CHANGE: ...` → bumps **major**.

### Scope (opcional pero recomendado)

Scope = REQ o feature: `feat(req-08): add agent supervisor pattern`, `fix(rls): policy missing on secrets`, `docs(rules): expand security.md`.

### Ejemplos

```
feat(req-02): implement passwordless OTP login

Add OTP-based auth flow with RUT/email identifier.
Email channel via SMTP (mailpit dev).

Refs: issue-02.7
```

```
fix(seeders): handle is_user_modified flag correctly

Was overwriting customizations on each seed run.

Closes #42
Co-Authored-By: <not allowed per CLAUDE.md>
```

```
feat(req-08)!: change agent.subordinates schema to require slugs

BREAKING CHANGE: agents.subordinates ahora es TEXT[] con slugs,
no UUIDs. Migration 000XYZ aplica conversión automática.

Refs: issue-08.6
```

## NO se permite

- `Co-Authored-By` o AI attribution en commits (per CLAUDE.md global)
- Force push a `main`
- Commits con secrets/credenciales
- Mensajes vagos: `update`, `fix bug`, `wip`, `asdf`
- `--no-verify` o skipping hooks salvo emergency con post-mortem

## SemVer

Format: `vMAJOR.MINOR.PATCH` (ej. `v0.1.0`, `v1.2.3`).

- `0.x.y`: pre-1.0 — breaking permitido en minor; usar para alfa/beta
- `1.0.0` → `1.x.y`: estable; breaking solo en major bump
- Pre-release: `v1.0.0-rc.1`, `v1.0.0-beta.2`
- Tags firmadas con cosign (issue-19.2)

### Reglas de bump

- Breaking change → major bump (`v1.x.y` → `v2.0.0`)
- Nueva feature backwards-compatible → minor (`v1.2.3` → `v1.3.0`)
- Fix backwards-compatible → patch (`v1.2.3` → `v1.2.4`)
- En 0.x.y: breaking → minor bump (`v0.5.x` → `v0.6.0`)

### Componentes con SemVer propio

- Binary `domain-mcp` — main SemVer
- API HTTP — versionado en URL (`/api/v1/`); issue-13.8 maneja sunset
- SDK Python (`domain-sdk`) — SemVer independiente, refleja API major
- SDK TypeScript (`@domain/sdk`) — idem
- SDK Go (`github.com/domain/sdk-go`) — idem
- Helm chart `domain/domain-mcp` — SemVer independiente (chart version != app version per Helm spec)
- DB schema — migration version (numérica, no SemVer)

## CHANGELOG.md

- Generado AUTOMÁTICAMENTE a partir de Conventional Commits via `goreleaser` o `git-cliff`
- Formato Keep a Changelog: secciones Added, Changed, Deprecated, Removed, Fixed, Security
- Cada release tag aparece en CHANGELOG con fecha
- Unreleased section al tope durante desarrollo
- NO se edita a mano excepto curation pre-release

## Tags

```bash
# Pre-release
git tag -a v0.1.0-rc.1 -m "Release candidate 1 of v0.1.0"
git push origin v0.1.0-rc.1   # dispara CI release draft

# Release
git tag -a v0.1.0 -m "Release v0.1.0 — Foundation alpha"
git push origin v0.1.0        # dispara CI release final + publish artifacts
```

## Workflow día-a-día

1. `git checkout -b feat/short-name main`
2. Commits frecuentes con Conventional Commits
3. `git push -u origin feat/short-name`
4. Abrir PR (template `.github/PULL_REQUEST_TEMPLATE.md`)
5. CI verde + code review → merge squash (mensaje squash respeta Conventional)
6. Branch auto-delete tras merge
7. Tag periódico según release cadence (típicamente sprint biweekly)

## Commit hooks (opcional, recomendado)

- `pre-commit`: `make lint` rápido (golangci-lint sobre archivos changed)
- `commit-msg`: validate Conventional format con `commitlint`
- `pre-push`: `go test -short ./...`

Instalar via `pre-commit` framework o `.git/hooks/` versionados.

## CI enforcement

- `.github/workflows/ci.yml` valida formato Conventional via action `wagoid/commitlint-github-action`
- PRs con mensaje no-conventional → CI fail
- CHANGELOG generation step en release workflow

## Releases automáticos

- Tag `vX.Y.Z` push → workflow `release.yml` (issue-19.2):
  1. Build multi-arch binaries con goreleaser
  2. Sign con cosign keyless
  3. SBOM con syft
  4. CHANGELOG section desde commits desde último tag
  5. GitHub Release con artifacts attached
  6. Docker image push (issue-19.3)
  7. Helm chart push OCI (issue-24.1)
  8. SDKs publish (issue-22.* en su workflow propio)

## Documentación de cambios

- Cada PR actualiza `CHANGELOG.md` Unreleased section (manual durante desarrollo)
- En release: Unreleased → vX.Y.Z con fecha
- Breaking changes documentados en `docs/migrations/v<from>-to-v<to>.md`
