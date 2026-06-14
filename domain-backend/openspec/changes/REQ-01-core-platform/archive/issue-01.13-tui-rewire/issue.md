# HU-01.13 — TUI rewire: auto-.env + errores limpios

## Problema (output real del user)

User corrio `domain` → TUI → Install → "Install failed: exit status 1" +
"config validation: DOMAIN_DATABASE_URL is required".

Diagnostico:
1. La TUI lanzo sub-process `domain install --mode local --non-interactive`
2. El sub-process arranco y fallo en `config.Load()` porque faltaba `.env`
3. El TUI mostro "Install failed: exit status 1" — el error real (DOMAIN_DATABASE_URL required) se perdio en el wrap de exec.Command

3 bugs reales:
- **No hay auto-bootstrap de `.env`**: la spec de HU-01.11 lo prometia (`ensureLocalEnv`) pero nunca lo implemente en el codigo de install_cli.go
- **Error wrapping con `exit status 1`**: la TUI usa `cmd.Run()` que solo retorna exit code, no el stderr del sub-process
- **Sub-process cuando ya estamos en un proceso Go**: anti-patron. Estamos en `domain` (TUI), hacemos exec.Command de `domain install` (CLI). Doble proceso, doble memoria.

## Goal (scope de esta HU, quirurgico)

Resolver el 90% del problema con 2 cambios minimos:

1. **Auto-.env**: si `.env` falta y `.env.example` existe, copiarlo ANTES de llamar al sub-process. Asi, cuando el sub-process arranca, ya tiene config.
2. **Error propagation con contexto**: capturar stderr del sub-process y mostrarlo al user, no solo el exit code. Asi si el sub-process falla, el user ve QUE fallo.

## Out of scope (queda para HU-01.14)

- **Eliminar el sub-process completamente** y llamar a la logica directa. Esto requiere mover `cmd/domain/install_cli.go` a `internal/installer/cli.go` (paquete reusable). Es un refactor mas grande.
- Tests E2E en TTY real (sigue pendiente por falta de /dev/ptmx).
- Hybrid mode con per-service prompts (sigue sin implementar).

## Acceptance Criteria

### AC1: Auto-.env en install feature
- Antes de llamar al sub-process, la TUI chequea si `.env` existe
- Si no existe y `.env.example` existe, copia `.env.example` → `.env`
- Si no existe ninguno de los dos, error claro: ".env.example not found in current directory; are you in the domain project root?"
- Muestra un step mas en InstallProgress: "Bootstrap .env" con status ok/skipped/failed

### AC2: Error propagation
- Si sub-process falla, la TUI muestra el stderr real (no "exit status 1")
- Si sub-process escribe a stderr (e.g., "config validation: DOMAIN_DATABASE_URL is required"), la TUI lo muestra
- Limite: 4KB de stderr (no无限的)

### AC3: Smoke test
- Sin `.env`, correr `domain install --mode local` desde la TUI (no testeable sin TTY, pero verificable via CLI `domain install` con sub-process flag)

## Implementation plan (3 commits atomicos)

### Commit 1/3: ensureLocalEnv en install_cli.go
- cmd/domain/install_cli.go: nueva funcion `ensureLocalEnvFile()` que:
  - Stat `.env`; si existe, return nil (skip)
  - Si no, Stat `.env.example`; si no existe, return error claro
  - Si existe, ReadFile + WriteFile `.env`
- Wirear en `runInstall` ANTES de `config.Load()` para que la config este disponible
- Tests: TestEnsureLocalEnvFile_Copies / _Skips / _FailsIfExampleMissing

### Commit 2/3: error propagation en TUI install runner
- internal/tui/features/install/runner.go: cambiar `cmd.Run()` por logica que captura stderr
- Nueva struct RunResult { ExitCode int, Stderr string }
- Update install.go: en runResultMsg, incluir Stderr
- View: si err, mostrar Stderr (no "Install failed: exit status 1")

### Commit 3/3: smoke test + state.yaml implemented + archive
- Manual smoke test: clonar, build, run, verificar auto-.env
- state.yaml: implemented
- archive/

## Archivos a tocar

```
openspec/changes/REQ-01-core-platform/issue-01.13-tui-rewire/  (nuevo)
cmd/domain/install_cli.go                        (commit 1, 2)
cmd/domain/install_cli_test.go                   (commit 1)
internal/tui/features/install/runner.go          (commit 2)
internal/tui/features/install/install.go         (commit 2)
internal/tui/features/install/install_test.go    (commit 2)
```

## Riesgos

| Risk | Mitigation |
|------|------------|
| Auto-.env pisa .env existente | Stat primero, skip si existe |
| stderr del sub-process tiene 10MB | Limitar a 4KB con io.LimitReader |
| User tiene .env con secrets reales que no quiere pisar | El copy solo ocurre si .env NO existe |
| El sub-process sigue siendo suboptimo | Out of scope para HU-01.13; queda para HU-01.14 |
