# HU-01.11 — Install end-to-end (clone + un comando)

## Problema

Hoy `domain install` (HU-01.10) requiere que el user primero:
1. Clone el repo
2. `go build` el binario
3. Copie `.env.example` → `.env` a mano
4. Levante Docker (o configure DSN cloud)
5. Recién ahí corra `domain install --mode local`

Eso es 5 pasos manuales. **No es "clone y listo"** — se rompe el contrato
del usuario de "ejecutar un instalador y que decida todo adentro".

## Goal

Una vez implementado:

```bash
curl -fsSL https://raw.githubusercontent.com/<org>/domain/main/install.sh | bash
domain   # sin args → wizard que decide todo
# o:
domain install   # alias explícito, sin flags → mismo wizard
# o:
domain install --mode local --non-interactive   # CI/scripted
```

El wizard es interactivo y pregunta 4 cosas:
1. **Deployment mode**: `local` (docker) / `cloud` (DSN) / `hybrid` (per-service)
2. **Base URL** (default: `http://localhost:8000`)
3. **¿Archivar archivos .md a BD?** (init — backup de CLAUDE.md, .claude/**, etc.)
4. **¿Configurar opencode MCP server?** (requiere API key existente o genera una)

Después corre los 5 steps existentes con `InstallProgress`:
detect → backup → migrate → seed → deploy-mode → (init?) → (opencode?)

Y al final, un summary con `ok=N skipped=N failed=N`.

## Acceptance Criteria

### AC1: Install one-liner
- `install.sh` en la raíz del repo (estilo `ptools`/`personal-tools`)
- Clona a `~/.local/share/domain` (o respeta `$DOMAIN_INSTALL_DIR`)
- Compila con `go build -o $HOME/go/bin/domain ./cmd/domain` (o `$DOMAIN_INSTALL_DIR`)
- `install.sh` debe ser **idempotente**: si ya existe el repo, hace `git pull --ff-only`
- Detecta si falta `git` o `go` y falla con mensaje claro

### AC2: Auto-bootstrap en `install --mode local`
- Si `.env` no existe Y `.env.example` existe, copiar `.env.example` → `.env` automáticamente
- Si Docker NO está corriendo, falla con error claro antes de `docker compose up`
- Si Docker SÍ está corriendo, `docker compose up -d` antes del migrate
- El copy de `.env.example` es un step más del wizard (status: ok/skipped)

### AC3: Wizard interactivo
- `domain` sin args + `domain install` sin `--mode` → entra en wizard
- 4 preguntas via `bufio.Scanner` (NO bubbletea — el codebase ya decidió no usarlo)
- Cada pregunta tiene un default sensato (Enter = default)
- Modo `--non-interactive` (o `-y`) salta todas las preguntas
- Al final muestra el banner de `InstallProgress` con los 5 steps

### AC4: Idempotencia
- `domain install` corrido 2 veces seguidas debe ser no-op la segunda
- `domain install` después de un `update` debe skip migrate/seed (ya están al día)
- `domain install` sobre un server ya bootstrapped debe skip el modo (solo re-imprime state)

### AC5: Backups y recovery
- Backups automáticos de credentials.json, .env, opencode.json antes de cualquier mutación
- Si el user tiene `.env` viejo y el install crea uno nuevo, el viejo queda en `.env.bak.<ts>`
- `domain restore <bak-path>` permite rollback puntual

## Out of scope

- Auto-update del binario (lo tiene `ptools` con `git pull` en cada launch, NO lo agregamos acá — `install.sh` lo cubre y se re-corre manualmente)
- Detección de OS (asumimos Linux/macOS; `install.sh` falla claro en Windows)
- Setup de systemd/init.d para auto-start del server (HU futura)
- TUI pesada con bubbletea (decidido NO usar; `bufio.Scanner` es suficiente para 4 prompts)

## Decisiones de diseño

### D1: NO bubbletea
El codebase no usa charmbracelet/bubbletea (grep lo confirmó: no hay imports).
Usar bubbletea solo para 4 prompts sería over-engineering y agregaría una
dependencia pesada. `bufio.Scanner` + `InstallProgress` es coherente.

### D2: install.sh separado del binario
Igual que `ptools`: el script es dumb (clona, compila), el binario es smart
(wizard, detección, config). Separación clara de responsabilidades.

### D3: `domain` sin args = install
Hoy `domain` sin args cae al `default:` del switch que imprime `printUsage()`.
**Cambio:** si no hay args, redirige a `runInstall(nil)` con `nonInter=false`.
Esto hace que `domain` solo sea el instalador (es el flujo principal del user nuevo).

Alternativa considerada: dejar `domain` → help, y forzar al user a escribir
`domain install`. **Rechazada** porque contradice el goal de "1 comando".

### D4: `domain install` y `domain` sin args son equivalentes
Cualquiera de los dos arranca el wizard. Los flags de `domain install` se
respetan en ambos casos.

## Implementation plan (3 commits atómicos)

### Commit 1/3: Auto-bootstrap en local mode
- `install_cli.go`: nueva función `ensureLocalEnv()` que:
  - Verifica `.env` existe; si no, copia `.env.example` → `.env`
  - Verifica Docker está corriendo (no solo `DockerAvailable` sino `DockerRunning`)
  - Si algo falla, retorna error claro
- `internal/cli/install/state.go`: NO TOCAR — el campo `DockerRunning` ya existe
- Test unitario: `ensureLocalEnv` con `.env` faltante, con `.env` presente, con Docker apagado

### Commit 2/3: Wizard interactivo completo
- `install_cli.go`: nueva función `runInstallWizard()` que:
  - Pregunta 4 cosas (mode, base-url, init, opencode)
  - Llama a `runInstall(args)` con los flags populados
- `main.go`: `default:` del switch → `os.Exit(runInstall(nil))` (D3)
- Test: `runInstallWizard` con stdin mockeado (bufio.Scanner sobre `bytes.Buffer`)

### Commit 3/3: install.sh + state.yaml implemented
- `install.sh` en la raíz del repo (basado en `ptools/install.sh`)
- Cross-link desde `docs/GETTING_STARTED.md` con la sección "Quick install"
- `state.yaml`: `proposed` → `implemented` con la lista de commits
- Mover HU a `archive/`

## Archivos a tocar

```
openspec/changes/REQ-01-core-platform/issue-01.11-installer-end-to-end/  (nuevo)
  issue.md
  design.md
  proposal.md
  tasks.md
  state.yaml
cmd/domain/install_cli.go                    (commit 1, 2)
cmd/domain/main.go                           (commit 2)
internal/cli/install/...                     (no tocar — InstallState ya tiene lo necesario)
install.sh                                    (commit 3 — nuevo, raíz)
docs/GETTING_STARTED.md                       (commit 3 — sección Quick install)
```

## Riesgos

- **R1:** `install.sh` modifica el PATH del user. Mitigación: NO modificar
  `.bashrc`/`.zshrc`. Solo mostrar mensaje claro con instrucciones de añadir.
- **R2:** `go build` falla si el user tiene una versión vieja de Go.
  Mitigación: chequeo de versión mínima (Go 1.22+) en `install.sh`.
- **R3:** `docker compose up` puede tardar mucho o colgar. Mitigación: ya
  existe `WaitHealthy` con timeout de 90s. Solo propagar el error bien.
- **R4:** Re-correr `install` sobre un install viejo puede pisar configs.
  Mitigación: backups automáticos + flag `--no-backup` para override.
