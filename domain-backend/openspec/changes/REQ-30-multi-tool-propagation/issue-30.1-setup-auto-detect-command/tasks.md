# Tasks: issue-30.1-setup-auto-detect-command

## Backend

- [ ] **T1**: Crear paquete `internal/cli/setup/autodetect/` con:
  - `detect.go` — `type State int; const ( StateNone State = iota;
    StateClaudeMDOnly; StateMCPJSONOnly; StateOpenCodeConfigOnly;
    StateAllPresent )`. Función `Detect(path string) (State, error)`
  que escanea los 6 paths canónicos (`.claude/`, `.opencode/`,
  `.cursor/`, `.mcp.json`, `AGENTS.md`, `CLAUDE.md`, `opencode.json`).
  - `actions.go` — `type Action struct { Type, Path, Target, Key
  string; BeforeHash, AfterHash string }`. Función
  `Apply(path string, state State) ([]Action, error)` que retorna la
  lista de acciones aplicadas (o no-ops). Idempotente: si el estado
  deseado ya existe, retorna slice vacío.
  - `manifest.go` — `type Manifest struct { Version int; DomainVersion
  string; AppliedAt time.Time; Actions []Action }`. Funciones
  `Read(path) (*Manifest, error)`, `Write(path string, m
  *Manifest) error`, `Append(path string, action Action) error`.

- [ ] **T2**: Implementar `RunAutoDetect(args []string) int` en
  `cmd/domain/setup.go` (nuevo archivo) o agregar al setup existente.
  Parsea flags: `<path>` (required, default=$PWD), `--quiet` (bool,
  default=false), `--dry-run` (bool, default=false).

- [ ] **T3**: Wire en `main.go` switch — `case "setup": runSetup(...)`
  debe delegar a `RunAutoDetect` cuando el primer arg es
  `auto-detect`. Mismo patrón que `domain setup opencode`.

- [ ] **T4**: Symlink action: usar `os.Symlink(target, linkPath)` con
  permisos best-effort. Si el symlink ya existe y apunta al mismo
  target, skip (no-op). Si apunta a OTRO target, error claro.

- [ ] **T5**: JSON upsert action: helper `UpsertJSONKey(path, dottedKey,
  value)` que lee el JSON, hace `set` recursivo en la dottedKey (e.g.
  `mcpServers.domain`), y escribe de vuelta. Preserva formatting (2
  spaces) y el resto de las keys intactas. Idempotente: si el value
  ya está, skip.

- [ ] **T6**: Backups: cada acción de escritura (symlink, json upsert)
  debe respaldar el archivo previo en `.bak-<ts>` antes de modificar.
  Reusar `install.BackupFile(path)`.

- [ ] **T7**: Manifest fallback: si `<path>/.domain/` no se puede crear
  (permisos), escribir a `~/.config/domain/orphan-manifests/<basename
  (path)>.json` con un warning. La idea: nunca perder el rastro de
  cambios por un permission issue.

## Tests

- [ ] **T-unit-1**: `TestDetect_StateNone**` — tempdir vacío →
  `Detect` retorna `StateNone`.
- [ ] **T-unit-2**: `TestDetect_StateClaudeMDOnly**` — tempdir con
  `CLAUDE.md` solo → `Detect` retorna `StateClaudeMDOnly`.
- [ ] **T-unit-3**: `TestDetect_StateMCPJSONOnly**` — tempdir con
  `.mcp.json` solo → `Detect` retorna `StateMCPJSONOnly`.
- [ ] **T-unit-4**: `TestApply_Symlink**` — tempdir con CLAUDE.md,
  Apply → AGENTS.md es symlink a CLAUDE.md; manifest tiene 1 action
  type=symlink.
- [ ] **T-unit-5**: `TestApply_JSONUpsert**` — tempdir con `.mcp.json`
  con `{opsx: ...}`, Apply → `.mcp.json` ahora tiene `{opsx: ...,
  domain: ...}`; manifest tiene 1 action type=json_upsert.
- [ ] **T-unit-6**: `TestApply_Idempotent**` — Apply 2 veces → segundo
  Apply retorna slice vacío de acciones; manifest tiene 1 action
  (no duplica).
- [ ] **T-unit-7**: `TestApply_DryRun**` — `Apply` con flag dry-run no
  toca archivos, pero retorna las acciones que HARÍA.
- [ ] **T-e2e-1**: `TestRunAutoDetect_FullProject**` — tempdir con
  mezcla (CLAUDE.md + .mcp.json con opsx) → `RunAutoDetect` → exit 0,
  AGENTS.md es symlink, .mcp.json tiene domain, manifest escrito.
- [ ] **T-sabotaje**: Comentar el check "if already applied, skip" en
  `Apply` → correr T-unit-6 (idempotencia) → DEBE ver duplicados
  (segundo AGENTS.md symlink falla, segundo .mcp.json upsert crea
  duplicado o pisa) → restaurar check → test verde. Documentar
  sabotaje en commit body.
