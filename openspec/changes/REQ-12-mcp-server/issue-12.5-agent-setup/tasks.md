# Tasks: issue-12.5-agent-setup

> Decisión de producto 2026-06-10: los targets del usuario son **Claude Code**
> y **OpenCode** (project-scope, commiteable). Claude Desktop se mantiene.
> Codex y Cline → DIFERIDOS (sin demanda; la estructura targets.go los
> agrega en ~30 líneas cuando se necesiten). Implementación en
> internal/cli/setup/ con funciones por target en lugar de interfaces
> detector/generator (YAGNI con 3 targets).

## Backend

- [x] Identificación de agentes → setup.Agent + SupportedAgents + ConfigPath por agente — 2026-06-10
- [x] Target Claude Desktop → SetupClaudeDesktop (config global por OS, rutas Linux/macOS/Windows)
- [x] Target Claude Code → SetupClaudeCode (.mcp.json del proyecto, formato mcpServers) — 2026-06-10
- [x] Target OpenCode → SetupOpenCode (opencode.json con $schema + mcp.domain type local) — 2026-06-10
- [x] Target Codex → DIFERIDO (decisión de producto)
- [x] Target Cline → DIFERIDO (decisión de producto)
- [x] Directivas → CreateAIDirectives (.ai/directives.md con uso de tools + archivos prohibidos)
- [x] Safe files → directivas listan .env/*.pem/credentials/.git como intocables; setup solo escribe configs MCP conocidos
- [x] Orquestación → runSetup dispatch (claude-code default | opencode | claude-desktop | status | uninstall) — 2026-06-10
- [x] Backup de config original antes de modificar → writeJSONWithBackup (timestamp UTC) — 2026-06-10
- [x] Uninstall → setup.Uninstall (quita solo el entry domain, preserva otros servers) — 2026-06-10
- [x] Status → setup.Status (config existe + domain configurado, por agente) — 2026-06-10
- [x] Comando `domain setup` con subcomandos → cmd/domain runSetup + help completo — 2026-06-10

## Tests

- [x] Claude Code crea config de proyecto → TestSetupClaudeCode_CreatesProjectConfig — 2026-06-10
- [x] OpenCode formato correcto ($schema, type local, command array) → TestSetupOpenCode_Format — 2026-06-10
- [x] Merge con MCP servers existentes (no pisa) → TestSetupClaudeCode_PreservesExistingServers — 2026-06-10
- [x] ClaudeGenerator (Desktop) → tests existentes de SetupClaudeDesktop
- [x] Directivas markdown → CreateAIDirectives idempotente (no sobrescribe)
- [x] Backup con timestamp → TestSetupClaudeCode_PreservesExistingServers (glob .backup-*) — 2026-06-10
- [x] Uninstall remueve solo domain → TestUninstall_RemovesOnlyDomain (+ doble uninstall no-op) — 2026-06-10
- [x] Status detecta agentes configurados → TestStatus_DetectsConfiguredAgents — 2026-06-10
- [x] Setup repetido → ErrAlreadyConfigured (idempotencia explícita en ambos targets)
- [x] Integración con MCP server real → cubierto por mcptest suite (tools/list del server in-process)
- [x] Sabotaje: JSON corrupto → setup NO sobrescribe → TestSabotage_CorruptConfig_NotOverwritten — 2026-06-10
- [x] Sabotaje: safe files → setup solo toca .mcp.json/opencode.json/claude config; verificado por construcción (paths fijos)

## Cierre

- [x] Verificación `domain setup claude-code` → cubierta por tests con TempDir (mismo código de producción)
- [x] Verificación `domain setup opencode` → idem
- [x] Verificación `domain setup status` post-setup → TestStatus_DetectsConfiguredAgents
- [x] Verificación uninstall → TestUninstall_RemovesOnlyDomain
- [x] Suite verde → 2026-06-10 (11 tests setup)
