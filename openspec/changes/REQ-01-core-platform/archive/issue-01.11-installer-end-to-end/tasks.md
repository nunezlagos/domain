# HU-01.11 — Tasks

## Commit 1/3: Auto-bootstrap en local mode

- [ ] **inst-001**: Crear `ensureLocalEnvFile()` en `install_cli.go`
  - Verifica `.env` existe
  - Si no, copia `.env.example` → `.env` con `os.ReadFile` + `os.WriteFile`
  - Retorna error si `.env.example` tampoco existe
- [ ] **inst-002**: Wirear en `handleDeploymentMode` para `ModeLocal`
  - Llamar `ensureLocalEnvFile()` ANTES de `install.StartDockerServices`
  - Es un step de `InstallProgress`
- [ ] **inst-003**: Test `TestEnsureLocalEnvFile_CopiesIfMissing`
- [ ] **inst-004**: Test `TestEnsureLocalEnvFile_SkipsIfExists`
- [ ] **inst-005**: Test `TestEnsureLocalEnvFile_FailsIfExampleMissing`
- [ ] **inst-006**: Verificar suite verde + build

## Commit 2/3: Wizard interactivo

- [ ] **inst-007**: Refactor `promptDeploymentMode` a `promptChoiceWithIO(in, out, ...)`
  - Acepta `io.Reader` y `io.Writer` para testear con buffers
  - Misma signature externa (default arg = `os.Stdin`, `os.Stderr`)
- [ ] **inst-008**: Nueva función `promptYesNoWithIO(in, defaultYes)`
- [ ] **inst-009**: Nueva función `promptLineWithIO(in, prompt, default)`
- [ ] **inst-010**: Nueva función `runInstallWizard(nonInter bool) int`
  - Si `nonInter`, delega directo a `runInstall(nil)`
  - Si no, hace 4 prompts y delega a `runInstall(args)`
- [ ] **inst-011**: Wirear `domain` sin args → `runInstallWizard(false)` en `main.go`
- [ ] **inst-012**: Wirear `domain install` sin `--mode` → `runInstallWizard(false)`
- [ ] **inst-013**: Test `TestPromptChoice_DefaultAccepted`
- [ ] **inst-014**: Test `TestPromptChoice_NumberSelected`
- [ ] **inst-015**: Test `TestPromptChoice_InvalidFallsBackToDefault`
- [ ] **inst-016**: Test `TestPromptYesNo_Defaults` (5 subcases)
- [ ] **inst-017**: Test `TestPromptLine_DefaultAccepted`
- [ ] **inst-018**: Test `TestRunInstallWizard_NonInteractiveSkipsPrompts`
- [ ] **inst-019**: Verificar binario: `domain` (sin args) muestra wizard, `domain install` también
- [ ] **inst-020**: Verificar `domain install --non-interactive` no muestra prompts
- [ ] **inst-021**: Verificar suite verde + build

## Commit 3/3: install.sh + archive

- [ ] **inst-022**: Crear `install.sh` en la raíz del repo (basado en ptools)
  - Detecta `git` + `go 1.22+`
  - Idempotente: clone o `git pull --ff-only`
  - Compila con `go build` → `$HOME/go/bin/domain`
  - Advierte si `$INSTALL_DIR` no está en PATH
  - Imprime: "Listo. Ejecuta: domain"
- [ ] **inst-023**: Hacer ejecutable: `chmod +x install.sh`
- [ ] **inst-024**: Sección "Quick install" en `docs/GETTING_STARTED.md`
- [ ] **inst-025**: `state.yaml`: `proposed` → `implemented`
- [ ] **inst-026**: Mover spec a `archive/`
- [ ] **inst-027**: Verificar suite verde + build
- [ ] **inst-028**: Commit + merge a main

## Total: 28 tasks, 3 commits
