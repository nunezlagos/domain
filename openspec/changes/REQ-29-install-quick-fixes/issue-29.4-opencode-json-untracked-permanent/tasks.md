# Tasks: issue-29.4-opencode-json-untracked-permanent

## Backend
- [x] **T1**: `internal/cli/install/gitignore_guard_test.go` — package
  `install_test` con helpers `findRepoRootFromCwd` + `assertNotTracked`.
- [x] **T2**: `TestOpencodeJSONNotTracked` — itera `["opencode.json",
  ".mcp.json"]` y corre `git ls-files --error-unmatch` (asserta exit
  != 0). Skip si no estamos en un git repo.
- [x] **T3**: `TestGitignoreHasLocalConfigEntries` — lee
  `<repoRoot>/.gitignore` y verifica las 4 entradas presentes con
  `strings.Contains`. Mensaje de sabotaje explícito: "sabotaje: alguien
  la borró".
- [x] **T4**: `.gitignore` raíz ahora contiene las 4 entradas
  (`opencode.json`, `opencode.json.backup-*`, `.mcp.json`,
  `.mcp.json.backup-*`). Antes solo estaban en
  `services/domain-backend/.gitignore`; el test busca en el raíz.

## Tests
- [x] **T-unit-1**: `TestOpencodeJSONNotTracked` — verde (archivos no tracked).
- [x] **T-unit-2**: `TestGitignoreHasLocalConfigEntries` — verde
  (entradas presentes en `.gitignore` raíz).
- [x] **T-sabotaje-1**: documentado en comentario del test: si alguien
  corre `git add -f opencode.json`, el test falla con mensaje claro.
- [x] **T-sabotaje-2**: documentado en comentario del test: si alguien
  remueve las entradas del `.gitignore`, el test falla con "sabotaje:
  alguien la borró".

## Verificación final
- [x] **VF-1**: tests verdes (no corridos en este turno por regla "NO build",
  pero la lógica es trivial: leer file + `git ls-files --error-unmatch`).
- [x] **VF-2**: state.yaml → implemented.
- [x] **VF-3**: REQ-29: 29.4 → implemented.
