# Tasks: issue-35.3-setup-primary-memory-detect-engram

## Backend

- [ ] **T1]: Crear `internal/cli/setup/primary_memory/catalog.go`:
  - `var knownMemoryProviders = map[string]bool{...}` con los
    providers conocidos (engram, mem0, memory, knowledge, recall,
    cognee, graphiti).
  - `LoadCatalog() (map[string]bool, error)`: lee
    `~/.config/domain/primary-memory-catalog.json` (override
    del hardcoded).

- [ ] **T2]: Crear `internal/cli/setup/primary_memory/detect.go`:
  - `Detect(agent, configPath string) ([]DetectedProvider, error)`.
  - Tipos de agente: "opencode" (opencode.json), "claude-code"
    (claude.json).
  - `DetectedProvider{Name, ConfigPath, IsMemory bool}`.
  - Parsing defensivo: si el JSON está malformado, retorna
    warning + lista vacía.

- [ ] **T3]: Crear `internal/cli/setup/primary_memory/disable.go`:
  - `Disable(configPath, providers []string) error`.
  - Estrategia: cambiar `command` a `false` (o `[]` si es
    array) en cada entry.
  - OpenCode format: `mcp.<name>.command = false` (o
    `mcp.<name>.command = ["false"]`).
  - Claude Code format: `mcpServers.<name>.command = false`.
  - Backup pre-cambio con `install.BackupFile`.
  - Manifest global entry (REQ-30.4).

- [ ] **T4]: Crear `internal/cli/setup/primary_memory/reactivate.go`:
  - `Reactivate(configPath) error`.
  - Lee el backup más reciente de la lista
    `<configPath>.bak.*` (ordenados por timestamp).
  - Restaura el archivo entero.

- [ ] **T5]: Wire en `cmd/domain/setup.go`:
  - Flags: `--primary-memory [--reactivate] [--yes]`.
  - Si `--primary-memory`:
    1. Detect providers de memoria.
    2. Si ninguno → "no other memory providers detected" + exit 0.
    3. Mostrar lista + preview.
    4. Si `--reactivate` → Reactivate.
    5. Else (disable) → si no `--yes`, prompt. Si acepta →
       Disable.

- [ ] **T6]: Manifest global integration (REQ-30.4):
  - En `disable`, agregar entry al manifest con
    `type: "memory_provider_disable"`, `providers_disabled`,
    `before_hash`, `after_hash`.
  - En `reactivate`, agregar `type: "memory_provider_reactivate"`.

- [ ] **T7]: Update help text de `domain setup`:
  ```
  --primary-memory    Detect and disable other memory MCP providers
                      (e.g. engram, mem0) to make domain the only
                      memory. Use --reactivate to undo.
  ```

## Tests

- [ ] `TestDetect_OpenCode_FindsEngram**` — opencode.json con
  engram + domain → Detect retorna [engram].
- [ ] `TestDetect_OpenCode_IgnoresFilesystem**` — opencode.json
  con filesystem + domain → Detect retorna [] (filesystem no es
  memory).
- [ ] `TestDetect_ClaudeCode_FindsMem0**` — claude.json con
  mem0 → Detect retorna [mem0].
- [ ] `TestDetect_NoMemoryProviders**` — solo domain → [].
- [ ] `TestDisable_ChangesCommandToFalse**` — Disable engram →
  el entry de engram en el JSON tiene `command: false`.
- [ ] `TestDisable_BackupCreated**` — Disable → el archivo
  `.bak.<ts>` existe y tiene el contenido original.
- [ ] `TestDisable_ManifestEntry**` — Disable → el manifest
  global tiene la entry con `providers_disabled: ["engram"]`.
- [ ] `TestReactivate_RestoresFromBackup**` — Disable +
  Reactivate → el archivo vuelve a su estado original.
- [ ] `TestIsIdempotent**` — Disable 2 veces → la 2da detecta
  que engram ya está deshabilitado y no hace nada.
- [ ] `TestCatalog_OverrideFromJSON**` — JSON override del
  catalog → el override gana sobre el hardcoded.
- [ ] `T-sabotaje`: Comentar la línea de `install.BackupFile` en
  Disable (sabotaje: no backup) → test `TestDisable_BackupCreated`
  DEBE FALLAR → restaurar backup → test verde. Documentar en
  commit body.
