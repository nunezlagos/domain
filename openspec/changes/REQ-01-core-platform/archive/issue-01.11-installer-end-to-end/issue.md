# HU-01.11 — Install end-to-end con TUI bubbletea + auto-install deps

## Problema (actualizado post-feedback)

Después de HU-01.10, `domain install` funciona pero requiere:
1. Que el user pre-instale Go, git, Docker (si --mode local)
2. Que el user copie `.env.example` → `.env` a mano
3. Que el user corra `docker compose up -d` antes del install
4. Conocimiento de flags (`--mode`, `--dsn`, `--non-interactive`)

**Feedback del user:** "el instalador debe verificar si tenemos todo lo necesario y si no instalarlo. win mac o linux y linux dependiendo de la distro. dentro de la tui del instalador deberia mostrar install update backups exit etc como personal tools."

**Decisiones tomadas (confirmadas con el user):**
- **Auto-install con prompt de confirmación** (no silencioso, no fail)
- **Detección de OS** (linux/darwin/windows) + **distro** (apt/dnf/pacman/apk/brew/choco)
- **TUI bubbletea** (estilo ptools: menu, alt-screen, lipgloss)
- **Loop post-acción** (vuelve al menu despues de cada accion, exit explicito)
- **Solo lo que el mode necesita** (cloud → no docker, local → sí)

## Goal

```bash
# Pre-requisito unico: bash + curl (o git)
curl -fsSL https://raw.githubusercontent.com/<org>/domain/main/install.sh | bash
# Script: chequea git+go, clona, compila, advierte si PATH
# (NUEVO) Tambien puede auto-instalar git+go si faltan

domain    # sin args → TUI bubbletea
```

**TUI flow:**

```
┌─ DOMAIN ─────────────────── v0.3.0 ─┐
│                                      │
│   > 1. Install                       │
│     2. Update                        │
│     3. Backups                       │
│     4. Exit                          │
│                                      │
│   [enter] select   [q] quit          │
└──────────────────────────────────────┘
```

Al elegir **1. Install**, nueva pantalla con wizard de 4 pasos + dep-check + InstallProgress.

Al elegir **3. Backups**, lista de backups con seleccion + restore.

Al elegir **4. Exit**, sale (equiv a `q` o Ctrl-C).

## Acceptance Criteria

### AC1: Dep detection + auto-install
- `domain` (sin args) o `domain install` chequea deps antes del wizard
- Chequea: `go` (>=1.22), `git`, `docker` (solo si mode=local)
- Si falta `go` o `git`: **auto-install con confirm**:
  ```
  Need to install Go 1.22+ on ubuntu (apt).
  Proceed? [Y/n]:
  ```
  - Si dice Y → ejecuta `sudo apt install -y golang-go`
  - Si dice n → aborta con mensaje claro
- Si falta `docker` (mode=local): mismo flujo
- Cloud mode: no chequea docker

### AC2: OS/distro detection
- `runtime.GOOS`: `linux` / `darwin` / `windows`
- Linux: parsea `/etc/os-release` → `ID=ubuntu|debian|fedora|arch|alpine|...`
- Mac: `brew` (asumido)
- Windows: `choco` o `winget` (asumido, no testeado en CI)

### AC3: TUI bubbletea
- Menu principal con 4 items: Install, Update, Backups, Exit
- Navegacion: flechas ↑↓ o j/k; Enter selecciona; q/Ctrl-C sale
- Alt-screen habilitado (como ptools)
- Colores con lipgloss
- Despues de cada accion (excepto Exit): vuelve al menu
- Bubbletea Init/Update/View idiomáticos (cumple `tea.Model`)

### AC4: Install flow dentro de TUI
- Step 1: chequea deps (muestra spinner "Checking Go... ✓")
- Step 2: pregunta mode (1/2/3) — si no-interactivo, default local
- Step 3: pregunta base-url (default http://localhost:8000)
- Step 4: pregunta init y/n (default n)
- Step 5: pregunta opencode y/n (default y)
- Step 6: corre los 5 steps existentes con InstallProgress
- Step 7: summary + vuelve al menu

### AC5: Update flow dentro de TUI
- Chequea deps (solo go, no docker)
- Corre `domain update` logic con progress
- Vuelve al menu

### AC6: Backups flow dentro de TUI
- Lista backups disponibles (de credentials.json, .env, opencode.json)
- User selecciona con flechas
- Opciones: Restore, Delete, Cancel
- Vuelve al menu

### AC7: Idempotencia
- Re-correr `domain` no rompe nada
- `domain install` despues de `domain update` skip lo que ya está al día

## Out of scope

- Auto-update del binario (install.sh lo cubre)
- Soporte Windows real (script detecta y falla claro, binario no testea en windows)
- Setup de systemd/init.d
- Modificación de `.bashrc`/`.zshrc` para PATH

## Decisiones de diseño

### D1: bubbletea SI
**Trade-off:** +8 deps (charmbracelet ecosystem) y TUI compleja.
**Beneficio:** UX coherente con ptools, navegacion fluida, colores.
**Decisión:** Sí, pero aislado en `internal/tui/`. Tests usan `teatest` (lib oficial) que mockea stdin/stdout.

### D2: TUI NO en CI
Los tests de TUI usan `teatest` que no requiere TTY real. Pero los tests
de la lógica (installer, deps, prompts) son unitarios puros.

### D3: Auto-install con SUDO
- `apt install`, `dnf install` requieren sudo
- El binario debe pedir la password si no hay NOPASSWD
- `brew install` NO requiere sudo (mac)
- `choco install` requiere admin (windows)
- El user confirma con prompt antes de invocar sudo

### D4: Menu loop
- Implementado como state machine: `stateMenu` ↔ `stateFeature`
- Feature retorna `tea.Msg` con `BackMsg{}` → vuelve al menu
- `Exit` o `q` o `Ctrl-C` → `tea.Quit` con `ExitMsg{}`

## Implementation plan (5 commits atómicos)

### Commit 1/5: internal/installer (dep detection + auto-install)
- `internal/installer/os.go`: `DetectOS() (OS, error)` con `runtime.GOOS` + `/etc/os-release` parse
- `internal/installer/pkgmanager.go`: `DetectPackageManager(os OS) (PkgManager, error)` mapea distro → apt/dnf/pacman/apk/brew/choco
- `internal/installer/deps.go`: `Check(deps []Dep) (Results, error)`, `Install(pm PkgManager, dep Dep, withConfirm func(msg string) bool) error`
- Tests: OS detection (linux+ubuntu, mac+darwin, windows), pkg manager mapping, install con confirm mockeado

### Commit 2/5: internal/tui shell (menu + loop)
- `internal/tui/menu/menu.go`: bubbletea `Model` con 4 items (Install/Update/Backups/Exit), `Update`/`View`/`Init`
- `internal/tui/styles/styles.go`: lipgloss styles (title, selected, muted, help)
- `internal/tui/app/app.go`: state machine menu ↔ feature, model root
- Tests: menu Init/Update con KeyMsg, View format, app transitions

### Commit 3/5: install feature dentro de TUI
- `internal/tui/features/install/install.go`: bubbletea Model para install (4 prompts + 5 steps + summary + back)
- Reuso de `ensureLocalEnv`, `InstallProgress` existente
- Tests: install flow con input mockeado, deps check, mode prompts

### Commit 4/5: update + backups features dentro de TUI
- `internal/tui/features/update/update.go`: bubbletea Model para update
- `internal/tui/features/backups/backups.go`: bubbletea Model para backups (list + restore/delete)
- Tests: update flow, backups list, restore selection

### Commit 5/5: wireup en main.go + tests + archive
- `cmd/domain/main.go`: `default:` del switch → si `len(args)==1` lanza TUI app
- `domain install` (legacy) sigue funcionando via CLI (no TUI)
- `domain update`, `domain restore` igual
- Tests: main dispatch (CLI mode no TUI)
- `state.yaml`: `proposed` → `implemented` + lista de commits
- Mover spec a `archive/`

## Archivos a tocar

```
openspec/changes/REQ-01-core-platform/issue-01.11-installer-end-to-end/  (nuevo)
cmd/domain/main.go                            (commit 5)
cmd/domain/install_cli.go                     (no tocar — CLI mode sigue)
internal/installer/                           (commit 1 — nuevo paquete)
  os.go
  pkgmanager.go
  deps.go
  *_test.go
internal/tui/                                 (commit 2, 3, 4 — nuevo paquete)
  menu/menu.go
  menu/menu_test.go
  styles/styles.go
  app/app.go
  app/app_test.go
  features/install/install.go
  features/install/install_test.go
  features/update/update.go
  features/update/update_test.go
  features/backups/backups.go
  features/backups/backups_test.go
go.mod / go.sum                               (commit 1 — bubbletea + lipgloss)
```

## Riesgos

| Risk | Mitigation |
|------|------------|
| `teatest` falla en CI sin TTY | Usar `tea.WithoutRenderer()` en tests |
| `sudo` no disponible en Docker/CI | Tests no ejecutan install real; usan mock `withConfirm` |
| `apt update` falla por network | Error claro con retry instructions |
| TUI cuelga en pipe (`domain | grep`) | Detectar si stdout es TTY; si no, fallback a CLI |
| Bubbletea conflicts con `installProgress` (que escribe a stderr) | TUI captura stderr o `InstallProgress` se desactiva en TUI mode |
| +8 deps aumentan tiempo de `go mod download` | Aceptable; deps son livianas y aisladas |
| Tests E2E de TUI son flaky | Usar `teatest` con timeouts generosos; skip en `-short` |
