# Tasks: issue-30.5-propagate-existing-projects-scan

## Backend

- [ ] **T1**: Crear paquete `internal/cli/setup/propagate/` con:
  - `scan.go` — `type ProjectInfo struct { Name, Path string;
    HasDomain bool; DomainManifestAt string; IAConfigs []string }`.
    Función `Scan(rootPath string) ([]ProjectInfo, error)` que itera
    1 nivel y clasifica cada subdir.
  - `format.go` — `FormatTable(infos []ProjectInfo) string` que
    renderiza tabla ASCII con columns: NAME | PATH | DOMAIN |
    IA_CONFIGS. Truncar a 80 chars width.
  - `propagate.go` — `Propagate(selected []ProjectInfo, dryRun bool)
    (success, failed int, errs []error)` que invoca
    `exec.Command("domain", "setup", "auto-detect", p.Path, "--quiet")`
    por cada uno, captura output + exit code.

- [ ] **T2**: Comando `domain setup propagate` en `cmd/domain/setup.go`:
  - Flags: `--scan <path>` (default false, overridea el
    `propagate-paths.json`), `--all` (no-interactive),
    `--max-depth <n>` (default 1), `--yes` (skip prompt extra).
  - Si `--scan`: solo corre Scan + FormatTable + exit 0.
  - Si no `--scan`: corre Scan + lista + prompt
    "propagate these? [comma-separated | all | none]" + Propagate.

- [ ] **T3**: Config de paths: leer
  `~/.config/domain/propagate-paths.json` con estructura
  `{"paths": ["~/Proyectos", "~/work"]}`. Si no existe, crear con
  default `["~/Proyectos"]`. Función
  `LoadPropagatePaths() ([]string, error)`.

- [ ] **T4**: Interactive prompt: usar el patrón de
  `promptDeploymentMode` (cmd/domain/install_cli.go:1013) — leer
  stdin, parsear respuesta, validar. Números 1-based separados por
  coma, "all", "none"/Enter.

- [ ] **T5**: Summary final: `propagated to N projects, M failed`
  con detalle de los que fallaron (path + exit code + stderr
  truncado).

- [ ] **T6**: Performance: usar `os.ReadDir` (no `filepath.Walk`) para
  scan nivel-1. Cachear `os.Stat` results si es posible (no requerido
  para 80 proyectos).

## Tests

- [ ] **T-unit-1**: `TestScan_EmptyDir**` — tempdir sin subdirs →
  `Scan` retorna slice vacío.
- [ ] **T-unit-2**: `TestScan_WithDomainManifest**` — tempdir con 3
  subdirs, 1 con `.domain/install-manifest.json` → retorna 3
  ProjectInfo, 1 con HasDomain=true.
- [ ] **T-unit-3**: `TestScan_DetectsIAConfigs**` — subdir con
  `opencode.json` y `.mcp.json` → IAConfigs=["opencode.json",
  ".mcp.json"].
- [ ] **T-unit-4**: `TestScan_OneLevelOnly**` — tempdir con
  subdir/subdir/otro (3 niveles) → Scan nivel-1 retorna solo el
  nivel intermedio, no el profundo.
- [ ] **T-unit-5**: `TestFormatTable**` — 3 ProjectInfo →
  FormatTable retorna string con header + 3 filas.
- [ ] **T-e2e-1**: `TestPropagate_AllFlag**` — tempdir con 3 proyectos
  sin domain + flag `--all --yes` → 3 sub-procesos `auto-detect`
  invocados, summary "propagated to 3".
- [ ] **T-e2e-2**: `TestPropagate_InteractiveSelects**` — mock stdin
  con respuesta "1,3" → solo 2 sub-procesos invocados.
- [ ] **T-e2e-3**: `TestPropagate_ContinuesOnFailure**` — 1 proyecto
  cuyo `auto-detect` retorna exit != 0 (mockeado con `bin/false` en
  el command) → summary reporta 2 success + 1 failed, no aborta.
- [ ] **T-e2e-4**: `TestScan_IsReadOnly**` — scan sobre tempdir con
  proyectos sin domain → NINGÚN archivo del tempdir cambia (mtime,
  hash, contenido).
- [ ] **T-sabotaje**: Modificar `Scan` para que invoque
  `exec.Command("domain", "setup", "auto-detect", p.Path)` al final
  de la clasificación (sabotaje: scan mutates) → test e2e-4 DEBE
  FALLAR (los archivos se modifican) → restaurar Scan como read-only
  → test verde. Documentar en commit body.
