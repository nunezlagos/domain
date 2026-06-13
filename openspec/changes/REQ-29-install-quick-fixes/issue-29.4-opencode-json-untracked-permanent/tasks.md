# Tasks: issue-29.4-opencode-json-untracked-permanent

## Backend

- [ ] **T1**: Crear archivo
  `internal/cli/install/gitignore_guard_test.go` con el package
  `install_test` (externo, para que pueda ejecutar `exec.Command` sin
  contaminar el package install).

- [ ] **T2**: Implementar `TestOpencodeJSONNotTracked` que itera sobre
  `["opencode.json", ".mcp.json"]` y para cada uno corre
  `git ls-files --error-unmatch <path>`. Asserta `err != nil` (exit
  1 = no tracked). Skip con `t.Skip` si no estamos en un git repo
  (verificar con `git rev-parse --git-dir`).

- [ ] **T3**: Implementar `TestGitignoreHasLocalConfigEntries` que lee
  `../../.gitignore` (relativo al archivo de test) y verifica que las
  4 entradas (`opencode.json`, `opencode.json.backup-*`, `.mcp.json`,
  `.mcp.json.backup-*`) están presentes. Usar `strings.Contains` por
  entry, no regex, para evitar falsos positivos.

- [ ] **T4**: Verificar que ambos tests pasan en CI antes de mergear
  (correr `go test ./internal/cli/install/... -run 'TestOpencode|TestGitignore' -v`).

## Tests

- [ ] **T-unit-1**: `TestOpencodeJSONNotTracked` — el test pasa en
  el estado actual del repo.
- [ ] **T-unit-2**: `TestGitignoreHasLocalConfigEntries` — el test
  pasa en el estado actual del repo.
- [ ] **T-sabotaje-1**: Forzar `git add -f opencode.json` en un
  branch temporal + commit → correr el test → DEBE FALLAR con
  "opencode.json está tracked en git pero NO debería" → `git reset
  HEAD~1` + `rm --cached opencode.json` + restaurar → test verde.
- [ ] **T-sabotaje-2**: Remover las 4 líneas de `.gitignore`
  temporalmente + commit → correr `TestGitignoreHasLocalConfigEntries`
  → DEBE FALLAR → restaurar las líneas → test verde. Documentar el
  sabotaje en commit body.
