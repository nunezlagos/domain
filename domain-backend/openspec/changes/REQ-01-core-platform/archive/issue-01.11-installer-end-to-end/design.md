# HU-01.11 — Design

## Arquitectura

### Componentes nuevos

```
┌────────────────────────────────────────────────────────────────┐
│                     install.sh (root)                          │
│  - Detecta git, go                                             │
│  - Clona a ~/.local/share/domain (o git pull si existe)        │
│  - go build → $HOME/go/bin/domain                              │
│  - Imprime instrucciones de PATH                               │
└────────────────────────────────────────────────────────────────┘
                              │
                              ▼ (usuario corre `domain`)
┌────────────────────────────────────────────────────────────────┐
│                   cmd/domain (Go binary)                       │
│                                                                │
│  Sin args ──► runInstall(nil) ──► InstallWizard ──► 5 steps    │
│                                                                │
│  with args:                                                    │
│    install [flags] ──► parseInstallFlags ──► 5 steps           │
│    update [flags]  ──► runUpdate                               │
│    seed all        ──► runSeed                                 │
│    restore <path>  ──► runRestore                              │
│    server          ──► runServer                               │
│    migrate up|down ──► runMigrate                              │
│    ...                                                        │
└────────────────────────────────────────────────────────────────┘
```

### Wizard flow (runInstallWizard)

```
[1/4] Deployment mode?
      > 1) local (docker compose)
        2) cloud (DSN)
        3) hybrid (per-service)
      Choice [1]: 1

[2/4] Domain server URL?
      > [http://localhost:8000]: <enter>

[3/4] Archive .md files to DB (init)?
      > 1) yes
        2) no
      Choice [2]: <enter>

[4/4] Configure opencode MCP server?
      > 1) yes
        2) no
      Choice [1]: <enter>

==================================================
  Domain Install Wizard (issue-01.10)
==================================================

[1/5] Detecting state
    ✓ State: creds=no, docker=running, server=unreachable, first_run=yes, users=0
[2/5] Backing up configs
    ✓ 0 backed up, 3 skipped
[3/5] Applying migrations
    ✓ schema up to date
[4/5] Running seeders
    ✓ all catalogs at target version
[5/5] Deployment mode: local
    ✓ mode=local configured

Summary:
  ok=5 skipped=0 warning=0 failed=0 (total=5)
```

### ensureLocalEnv (commit 1)

```go
// ensureLocalEnv prepara el entorno para install --mode local.
// Idempotente: corre 1 vez, no-op si ya esta preparado.
func ensureLocalEnv(progress *InstallProgress) error {
    progress.StartStep("Preparing .env")
    if _, err := os.Stat(".env"); err == nil {
        progress.EndStep(StepSkipped, ".env already exists")
    } else {
        if _, err := os.Stat(".env.example"); err != nil {
            progress.EndStep(StepFailed, ".env.example not found (clone may be broken)")
            return errors.New(".env.example missing")
        }
        data, err := os.ReadFile(".env.example")
        if err != nil {
            progress.EndStep(StepFailed, err.Error())
            return err
        }
        if err := os.WriteFile(".env", data, 0o600); err != nil {
            progress.EndStep(StepFailed, err.Error())
            return err
        }
        progress.EndStep(StepOK, ".env created from .env.example")
    }
    return nil
}
```

### runInstallWizard (commit 2)

```go
// runInstallWizard muestra 4 prompts y delega a runInstall.
// Si nonInteractive, usa defaults sin preguntar.
func runInstallWizard(nonInter bool) int {
    if nonInter {
        return runInstall(nil)
    }
    // 4 preguntas con bufio.Scanner
    mode := promptChoice("Deployment mode", []string{"local", "cloud", "hybrid"}, "local")
    baseURL := promptLine("Domain server URL", "http://localhost:8000")
    doInit := promptYesNo("Archive .md files to DB (init)?", false)
    doOpencode := promptYesNo("Configure opencode MCP server?", true)
    
    args := []string{
        "--mode", mode,
        "--base-url", baseURL,
    }
    if !doInit {
        args = append(args, "--no-init")
    }
    if !doOpencode {
        args = append(args, "--no-opencode")
    }
    return runInstall(args)
}

func promptChoice(question string, options []string, defaultOpt string) string {
    fmt.Fprintf(os.Stderr, "%s\n", question)
    for i, opt := range options {
        fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, opt)
    }
    fmt.Fprintf(os.Stderr, "Choice [%s]: ", defaultOpt)
    line, _ := readLine()
    line = strings.TrimSpace(line)
    if line == "" {
        return defaultOpt
    }
    n, err := strconv.Atoi(line)
    if err != nil || n < 1 || n > len(options) {
        return defaultOpt
    }
    return options[n-1]
}

func promptYesNo(question string, defaultYes bool) bool {
    def := "Y/n"
    if !defaultYes {
        def = "y/N"
    }
    fmt.Fprintf(os.Stderr, "%s [%s]: ", question, def)
    line, _ := readLine()
    line = strings.ToLower(strings.TrimSpace(line))
    if line == "" {
        return defaultYes
    }
    return line == "y" || line == "yes"
}
```

### main.go cambio (D3)

```go
// Antes:
default:
    fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
    printUsage()
    os.Exit(2)

// Despues:
default:
    // Sin args o subcomando desconocido → wizard de install.
    // Esto hace que `domain` solo sea el instalador (D3).
    if len(os.Args) == 1 {
        os.Exit(runInstallWizard(false))
    }
    fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
    printUsage()
    os.Exit(2)
```

### install.sh (commit 3, basado en ptools)

```bash
#!/bin/bash
set -euo pipefail

REPO_URL="https://github.com/nunezlagos/domain.git"
SRC_DIR="${DOMAIN_INSTALL_SRC:-$HOME/.local/share/domain}"
INSTALL_DIR="${DOMAIN_INSTALL_DIR:-$HOME/go/bin}"
BINARY="domain"
MIN_GO_VERSION="1.22"

step() { echo -e "\n  -> $*"; }
ok()   { echo "  [ok] $*"; }
die()  { echo "  [x] $*" >&2; exit 1; }

# Chequeo Go
if ! command -v go >/dev/null; then
    die "Go $MIN_GO_VERSION+ not installed (https://go.dev/dl/)"
fi

# Chequeo version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
if [ "$(printf '%s\n' "$MIN_GO_VERSION" "$GO_VERSION" | sort -V | head -1)" != "$MIN_GO_VERSION" ]; then
    die "Go $MIN_GO_VERSION+ required, found $GO_VERSION"
fi

# Chequeo git
command -v git >/dev/null || die "git not installed"

step "Clonando repositorio..."
if [ -d "$SRC_DIR/.git" ]; then
    (cd "$SRC_DIR" && git pull --ff-only --quiet)
    ok "Source updated at $SRC_DIR"
else
    git clone --depth=1 "$REPO_URL" "$SRC_DIR"
    ok "Source cloned to $SRC_DIR"
fi

step "Compilando..."
mkdir -p "$INSTALL_DIR"
(cd "$SRC_DIR" && go build -o "$INSTALL_DIR/$BINARY" ./cmd/domain)
ok "Binary at $INSTALL_DIR/$BINARY"

# PATH warning
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *) echo "  [!] Add to PATH: export PATH=\"\$PATH:$INSTALL_DIR\"" ;;
esac

echo ""
echo "  Listo. Ejecuta: $BINARY"
echo ""
```

## Tests

### Unit tests (sin red)

```go
// cmd/domain/install_wizard_test.go

func TestPromptChoice_DefaultAccepted(t *testing.T) {
    in := bytes.NewBufferString("\n")  // Enter = default
    out := &bytes.Buffer{}
    got := promptChoiceWithIO("Q?", []string{"a", "b"}, "a", in, out)
    require.Equal(t, "a", got)
}

func TestPromptChoice_NumberSelected(t *testing.T) {
    in := bytes.NewBufferString("2\n")
    out := &bytes.Buffer{}
    got := promptChoiceWithIO("Q?", []string{"a", "b", "c"}, "a", in, out)
    require.Equal(t, "b", got)
}

func TestPromptChoice_InvalidFallsBackToDefault(t *testing.T) {
    in := bytes.NewBufferString("99\n")
    out := &bytes.Buffer{}
    got := promptChoiceWithIO("Q?", []string{"a", "b"}, "a", in, out)
    require.Equal(t, "a", got)
}

func TestPromptYesNo_Defaults(t *testing.T) {
    cases := []struct{
        input string
        defaultYes bool
        want bool
    }{
        {"", true, true},
        {"y\n", true, true},
        {"n\n", true, false},
        {"", false, false},
        {"y\n", false, true},
    }
    for _, tc := range cases {
        in := bytes.NewBufferString(tc.input)
        got := promptYesNoWithIO("Q", tc.defaultYes, in)
        require.Equal(t, tc.want, got)
    }
}
```

### Integration test (sin red, sin docker)

```go
// internal/cli/install/ensure_local_env_test.go (commit 1)

func TestEnsureLocalEnv_CopiesExampleIfMissing(t *testing.T) {
    dir := t.TempDir()
    t.Chdir(dir)  // requires Go 1.24+; si no, usar os.Chdir manual
    
    require.NoError(t, os.WriteFile(".env.example", []byte("KEY=value"), 0o600))
    require.NoError(t, ensureLocalEnvFile())  // helper que solo copia
    
    data, err := os.ReadFile(".env")
    require.NoError(t, err)
    require.Equal(t, "KEY=value", string(data))
}

func TestEnsureLocalEnv_SkipsIfEnvExists(t *testing.T) {
    dir := t.TempDir()
    t.Chdir(dir)
    
    require.NoError(t, os.WriteFile(".env", []byte("EXISTING=1"), 0o600))
    require.NoError(t, ensureLocalEnvFile())
    
    data, _ := os.ReadFile(".env")
    require.Equal(t, "EXISTING=1", string(data))  // unchanged
}
```

## Tradeoffs

### T1: bubbletea vs bufio.Scanner
- **bufio.Scanner**: ~50 líneas, 0 deps nuevas, suficiente para 4 prompts
- **bubbletea**: ~200 líneas, agrega charmbracelet ecosystem (varias deps),
  mejor UX visual pero overkill para 4 inputs

**Decisión:** bufio.Scanner. Coherente con el resto del codebase.

### T2: install.sh copia .env.example automáticamente?
- **Sí** (auto-copia en `ensureLocalEnv`): menos pasos para el user, 1 commit
  menos. Si el user tenía `.env` viejo, queda como `.env.bak.<ts>`.
- **No** (lo deja al user): más control, más fricción.

**Decisión:** Sí, auto-copia. Es idempotente y los backups cubren el rollback.

### T3: `domain` sin args = install vs help
- **install**: cumple el goal de "1 comando". Consistente con ptools.
- **help**: mantiene el patrón Unix de "sin args = help". Más predecible
  para users con experiencia Unix.

**Decisión:** install. El target es el user nuevo de Domain, no el
power user. ptools ya validó este patrón.

## Non-goals

- NO migrar de Postgres a otra DB
- NO agregar admin UI web (ya hay Adminer en docker-compose)
- NO auto-arrancar el server en background (eso es `domain server`, separado)
- NO modificar systemd/init.d
- NO soporte Windows (script falla claro)
