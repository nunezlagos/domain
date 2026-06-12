# HU-01.12 — Installer polish (4 gaps + E2E tests)

## Problema

Post-HU-01.11 mergeado a main, el user preguntó "¿están todos los bugs
resueltos?". Auditoría honesta reveló **4 gaps reales** que la suite
verde no detecta:

| # | Gap | Por qué es gap | Impacto |
|---|-----|----------------|---------|
| 1 | `install.sh` no existe | Spec de HU-01.11 lo prometía; quedó solo en papel | User tiene que correr `go build` a mano |
| 2 | `install` feature NO chequea docker en mode=local | Solo chequea go+git; docker check está en runtime, no en dep-check | Cloud user sin docker igual puede usar la TUI, pero local user sin docker lo descubre tarde |
| 3 | `domain` sin args NO lanza TUI | TTY detection hace fallback a printUsage | User tiene que escribir `domain tui` explícito. UX inconsistente con el goal "1 comando" |
| 4 | Tests de TUI son unitarios puros, no E2E | Bubbletea nunca se ejecutó en TTY real | Bugs de integración TTY (resize, signals, raw mode) no detectados |

## Goal

Una vez implementado:

```bash
# Un solo comando desde cero:
curl -fsSL https://raw.githubusercontent.com/<org>/domain/main/install.sh | bash
# install.sh chequea git+go, clona, compila

domain    # sin args → TUI (porque detecta TTY)
# o en CI / pipe:
domain install --mode cloud --dsn 'postgres://...'  # CLI mode
```

Y la TUI:
- Chequea **docker solo si mode=local** (ahorra friccion en cloud)
- Se ejecuta end-to-end con TTY simulado via teatest
- Falla con mensaje claro si `install.sh` no logra construir el binario

## Acceptance Criteria

### AC1: install.sh one-liner
- `install.sh` en la raíz del repo
- Patrón `ptools/install.sh`: chequea deps, clona a `~/.local/share/domain`,
  compila a `~/.local/bin/domain` o `$HOME/go/bin/domain`, advierte PATH
- Idempotente: si `~/.local/share/domain` ya existe, hace `git pull --ff-only`
- Chequeo de Go 1.22+ antes de clonar
- Modo `--uninstall` para borrar (nice-to-have, no en este commit)

### AC2: docker check por mode
- `internal/tui/features/install`: el dep-check agrega `DepDocker` solo si
  el user eligió `mode=local`
- Si `mode=cloud` o `mode=hybrid`, NO chequea docker (cloud trae su
  propio Postgres, hybrid lo resuelve en runtime)
- Mantiene la logica ya existente de `install.StartDockerServices` en
  runtime (no la duplica)

### AC3: `domain` sin args = TUI
- `cmd/domain/main.go`: si `len(os.Args) == 1` y `isTerminal(os.Stdin)`,
  lanza `runTUI(nil)` directamente
- Si `len(os.Args) == 1` y `!isTerminal`, fallback a `printUsage` (CLI)
- Mantiene `domain tui` como alias explícito

### AC4: Tests E2E con teatest
- `internal/tui/menu/menu_e2e_test.go`: usa `teatest` para simular TTY
- Test: arranca menu, envía `j` (down), `enter`, verifica que entró
  en feature install
- Test: arranca install feature, envía `q` en welcome, verifica que
  vuelve al menu (no sale)
- Test: detecta no-TTY gracefully (skip en `os.Getenv("CI")` o similar)
- Si teatest no funciona en nuestro setup, fallback a test manual
  via `tea.NewProgram(...)` con `tea.WithoutRenderer()`

## Out of scope

- Auto-update del binario al boot (HU-01.11 ya lo discutió; sigue fuera)
- Windows real testing (no hay ambiente; código es best-effort)
- systemd/init.d setup
- TUI con bubbletea v2 (quedamos con v1)

## Implementation plan (4 commits atómicos)

### Commit 1/4: install.sh en la raíz
- Basado en `personal-tools/install.sh` (mismo patrón)
- Chequeos: `git`, `go >= 1.22`, `bash`
- Clone a `$HOME/.local/share/domain` (override via `$DOMAIN_INSTALL_SRC`)
- Build a `$HOME/go/bin/domain` (override via `$DOMAIN_INSTALL_DIR`)
- PATH warning
- Ejecutable: `chmod +x install.sh`

### Commit 2/4: docker check condicional en install feature
- `internal/tui/features/install/install.go`: agregar `depsForMode(mode) []installer.Dep`
- En `checkDepsCmd`, usar `depsForMode(m.mode)` después del prompt
- Test: `TestDepsForMode_LocalIncludesDocker`, `TestDepsForMode_CloudExcludesDocker`

### Commit 3/4: `domain` sin args = TUI
- `cmd/domain/main.go`: al inicio de `main()`, si `len(os.Args) == 1`,
  llamar a `runTUI(nil)` si TTY, sino `printUsage`
- Eliminar el `printUsage + os.Exit(2)` actual
- Test: `TestMain_NoArgs_TTYLaunchesTUI` (mockear isTerminal)

### Commit 4/4: tests E2E con teatest
- Agregar dep `github.com/charmbracelet/bubbletea/.../teatest`
- `internal/tui/menu/menu_e2e_test.go`: test de menu end-to-end
- `internal/tui/features/install/install_e2e_test.go`: test de install
  flow (welcome → esc → vuelve al menu)
- Si teatest no se puede instalar o no funciona, fallback a tests
  manuales con `tea.NewProgram(..., tea.WithoutRenderer())`

## Archivos a tocar

```
openspec/changes/REQ-01-core-platform/issue-01.12-installer-polish/  (nuevo)
install.sh                                                      (commit 1)
internal/tui/features/install/install.go                        (commit 2)
internal/tui/features/install/install_test.go                   (commit 2)
cmd/domain/main.go                                              (commit 3)
cmd/domain/tui_cli.go                                           (commit 3)
internal/tui/menu/menu_e2e_test.go                              (commit 4)
internal/tui/features/install/install_e2e_test.go               (commit 4)
go.mod / go.sum                                                 (commit 4 - teatest)
```

## Riesgos

| Risk | Mitigation |
|------|------------|
| `teatest` requiere TTY real en CI | Skip con `t.Skip` si `$CI=true` |
| `teatest` agrega deps circulares | Verificar pre-check con `go get` antes de escribir tests |
| `install.sh` rompe el PATH | Mostrar warning, no modificar rc files |
| `domain` sin args cambia comportamiento de scripts | Documentar en printUsage; tests cubren el cambio |
| Windows no testeable | Mejor esfuerzo; documentar en README |
