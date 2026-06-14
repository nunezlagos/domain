# Tasks: issue-30.3-claude-code-sessionstart-hook

## Backend

- [ ] **T1**: Crear paquete `internal/cli/setup/claudehook/` con:
  - `settings.go` — `type Settings struct { Hooks *HooksConfig
    `json:"hooks,omitempty"`; Other map[string]any `json:",inline"` }
    + `type HooksConfig struct { SessionStart []Hook `json:"SessionStart,omitempty"` }`
    + `type Hook struct { Type string `json:"type"`; Command string
    `json:"command"` }`.
  - `reader.go` — `ReadSettings() (*Settings, []byte, error)` lee
    `~/.claude/settings.json`. Retorna settings + raw bytes (para
    diff). Si archivo no existe, retorna `&Settings{}, nil, nil`.
  - `hook_utils.go` — `HasDomainHook(s *Settings) bool` con regex
    `^domain setup auto-detect`; `AddDomainHook(s *Settings)
    *Settings` (append no-mutable).

- [ ] **T2**: `InstallClaudeHook(nonInteractive bool, autoAccept bool)
  (action string, err error)` en `claudehook.go`. Lógica:
  1. `s, raw, _ := ReadSettings()`.
  2. Si `HasDomainHook(s)` → return "already_installed", nil.
  3. `newS := AddDomainHook(s)`.
  4. Si `nonInteractive && !autoAccept` → return "skipped_noninteractive",
     nil.
  5. Imprimir diff de `raw` vs `marshal(newS)`.
  6. Si no autoAccept: prompt "apply? [y/N]".
  7. Si acepta: `install.BackupFile(settingsPath)` +
     `os.WriteFile(settingsPath, marshal(newS), 0600)`.
  8. Retornar "installed" + nil.

- [ ] **T3**: Wire en `runInstall` (cmd/domain/install_cli.go) como
  step 10.5 (entre Configure agents y Configure shell wrapper). Flag
  `--with-claude-hook` (sí sin prompt) y `--no-claude-hook` (skip).
  Default interactive: prompt con diff.

- [ ] **T4**: Comando standalone `domain setup claude-hook [--apply |
  --show]` que muestra el diff (`--show`) o aplica (`--apply`).
  Útil para el user que quiere revisar antes de install completo.

- [ ] **T5**: Integrar con manifest global (REQ-30.4): la entry de
  manifest para el hook tiene `{type: "claude_settings_merge", path:
  "~/.claude/settings.json", key: "hooks.SessionStart",
  before_hash, after_hash, command_added: "domain setup auto-detect
  \"$PWD\" --quiet"}`.

## Tests

- [ ] **T-unit-1**: `TestHasDomainHook_Fresh**` — settings vacío
  → `HasDomainHook` retorna false.
- [ ] **T-unit-2**: `TestHasDomainHook_AlreadyThere**` — settings con
  un Hook `{Type: "command", Command: "domain setup auto-detect ..."}`
  → `HasDomainHook` retorna true.
- [ ] **T-unit-3**: `TestHasDomainHook_PartialMatch**` — settings con
  `{Type: "command", Command: "echo domain setup auto-detect"}` →
  `HasDomainHook` retorna FALSE (no es un command que empieza con
  "domain setup auto-detect"). Cuidar la regex.
- [ ] **T-unit-4**: `TestAddDomainHook_PreservesOther**` — settings
  con `{"theme": "dark", "hooks": {"SessionStart": [{"type":
  "command", "command": "echo hi"}]}}` → `AddDomainHook` retorna
  settings con 2 hooks en SessionStart Y `theme: "dark"` preservado.
- [ ] **T-unit-5**: `TestReadSettings_Malformed**` — settings.json
  con JSON inválido → `ReadSettings` retorna warning + struct vacío
  (no falla), para que el user pueda decidir.
- [ ] **T-e2e-1**: `TestInstallClaudeHook_Applies**` — `InstallClaudeHook
  (false, true)` con settings inexistente → settings.json creado con
  chmod 600 + hooks.SessionStart con 1 entry; backup previo creado.
- [ ] **T-e2e-2**: `TestInstallClaudeHook_Merges**` — settings preexistente
  con otros hooks → `InstallClaudeHook` agrega el de domain sin
  perder los otros; diff muestra solo la adición.
- [ ] **T-e2e-3**: `TestInstallClaudeHook_Idempotent**` — install 2
  veces → la 2da retorna "already_installed" sin modificar archivo.
- [ ] **T-sabotaje**: Modificar el command generado a `echo "noop"`
  (sabotaje) → install con autoAccept → settings.json tiene el
  command sabotado → test e2e que assserta "manifest existe post-hook"
  DEBE FALLAR → restaurar command correcto → test verde. Documentar
  en commit body.
