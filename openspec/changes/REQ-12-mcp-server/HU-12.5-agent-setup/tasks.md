# Tasks: HU-12.5-agent-setup

## Backend

- [ ] `internal/setup/detector.go`: interface `AgentDetector` + struct `Agent` (Name, ConfigPath, Type)
- [ ] `internal/setup/detector_claude.go`: ClaudeDetector, rutas Linux/macOS
- [ ] `internal/setup/detector_opencode.go`: OpenCodeDetector
- [ ] `internal/setup/detector_codex.go`: CodexDetector
- [ ] `internal/setup/detector_cline.go`: ClineDetector
- [ ] `internal/setup/generator.go`: interface `ConfigGenerator` + struct `MCPConfig`
- [ ] `internal/setup/generator_claude.go`: ClaudeGenerator, produce JSON para claude_desktop_config.json
- [ ] `internal/setup/generator_opencode.go`: OpenCodeGenerator, produce JSON para .opencode/mcp.json
- [ ] `internal/setup/generator_codex.go`: CodexGenerator
- [ ] `internal/setup/generator_cline.go`: ClineGenerator
- [ ] `internal/setup/directives.go`: Template + generación de `.ai/directives.md`
- [ ] `internal/setup/safe_files.go`: Lista de patrones safe, checker
- [ ] `internal/setup/service.go`: Orquestador: detect → select → generate → apply
- [ ] `internal/setup/backup.go`: Backup de config original antes de modificar
- [ ] `internal/setup/uninstall.go`: Remover Domain de config del agente
- [ ] `internal/setup/status.go`: Reportar estado de configuración por agente
- [ ] `cmd/domain/setup.go`: Comando cobra `domain setup` con subcomandos

## Tests

- [ ] Test unitario: ClaudeDetector con filesystem mockeado
- [ ] Test unitario: OpenCodeDetector con `.opencode/mcp.json` existente
- [ ] Test unitario: ClaudeGenerator produce JSON exacto esperado
- [ ] Test unitario: ClaudeGenerator mergea con MCP servers existentes
- [ ] Test unitario: DirectivesGenerator produce markdown correcto
- [ ] Test unitario: SafeFilesChecker rechaza `.env`, `*.pem`, etc.
- [ ] Test unitario: SafeFilesChecker permite `.ai/directives.md`
- [ ] Test unitario: Backup crea copia con timestamp
- [ ] Test unitario: Uninstall remueve solo el entry de Domain
- [ ] Test unitario: Dry-run no escribe archivos
- [ ] Test de integración: setup + MCP server real, verificar tools/list
- [ ] Test de sabotaje: JSON corrupto en config → error parsing claro
- [ ] Test de sabotaje: Safe file `.env` modificado → test falla

## Cierre

- [ ] Verificación manual: `domain setup claude-code --dry-run` en macOS
- [ ] Verificación manual: `domain setup opencode --dry-run` en Linux
- [ ] Verificación manual: `domain setup status` después de setup
- [ ] Verificación manual: `domain setup uninstall --all --dry-run`
- [ ] Suite verde
