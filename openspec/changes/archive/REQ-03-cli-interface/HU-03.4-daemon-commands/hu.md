# HU-03.4-daemon-commands

**Origen:** `REQ-03-cli-interface`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** iniciar un servidor HTTP, exponer un servidor MCP, lanzar la TUI, y configurar agentes de IA
**Para** integrar memoria con otras herramientas y entornos de desarrollo

## Criterios de aceptación

```gherkin
Feature: serve command
  Scenario: Start HTTP server on default port
    Given el puerto 3000 está disponible
    When ejecuto memoria serve
    Then inicia el servidor HTTP en el puerto 3000
    And el output muestra "serving HTTP API on :3000"
    And el servidor responde en GET /health con {"status": "ok"}

  Scenario: Start HTTP server on custom port
    When ejecuto memoria serve 8080
    Then inicia el servidor HTTP en el puerto 8080
    And el output muestra "serving HTTP API on :8080"

  Scenario: Start HTTP server with token
    When ejecuto memoria serve --http-token mysecrettoken
    Then el servidor requiere el token en header X-API-Token para rutas no-públicas
    And GET /health no requiere autenticación

  Scenario: Port already in use
    Given el puerto 3000 ya está ocupado
    When ejecuto memoria serve 3000
    Then el comando retorna error "port 3000 already in use"
    And exit code es 1

  Scenario: Server responds to API requests
    Given el servidor está corriendo en el puerto 3000
    When envío GET /api/observations
    Then recibo una lista de observaciones en JSON
    And el status code es 200

Feature: mcp command
  Scenario: Start MCP server with default profile
    When ejecuto memoria mcp
    Then inicia el servidor MCP en stdio
    And responde al handshake de inicialización de MCP protocol
    And expone las herramientas definidas en el perfil default
    And el output muestra "MCP server ready (profile: default)"

  Scenario: MCP with specific tools profile
    When ejecuto memoria mcp --tools minimal
    Then solo expone las herramientas del perfil "minimal"
    And no expone herramientas del perfil "full"

  Scenario: MCP with --project
    When ejecuto memoria mcp --project memoria
    Then el servidor MCP opera solo sobre el proyecto "Domain"
    And las herramientas MCP filtran por ese proyecto

  Scenario: MCP stdio handshake
    Given el servidor MCP está escuchando en stdio
    When recibe un initialize request de MCP
    Then responde con server_capabilities y protocol_version
    And la conexión se establece correctamente

  Scenario: MCP tool call: save_observation
    Given el servidor MCP está activo
    When recibe un tools/call para "save_observation"
    Then crea una observación en la base de datos
    And responde con el ID de la observación creada

  Scenario: MCP tool call: search_memories
    Given el servidor MCP está activo
    When recibe un tools/call para "search_memories" con query "login"
    Then ejecuta una búsqueda FTS5 y retorna resultados

Feature: tui command
  Scenario: Launch TUI
    Given la terminal soporta 256 colores y tamaño mínimo 80x24
    When ejecuto memoria tui
    Then se inicia la interfaz TUI (bubbletea)
    And muestra la vista principal con observaciones recientes

  Scenario: TUI exits gracefully
    Given la TUI está corriendo
    When el usuario presiona Ctrl+C o 'q'
    Then la TUI termina gracefulmente
    And el estado de la terminal se restaura

  Scenario: TUI with small terminal
    Given el tamaño de terminal es menor a 80x24
    When ejecuto memoria tui
    Then muestra un error "terminal too small (min 80x24)"
    And no inicia la TUI

  Scenario: TUI terminal not supported
    Given la terminal no es interactiva
    When ejecuto memoria tui
    Then el comando retorna error "not a terminal"
    And exit code es 1

Feature: setup command
  Scenario: Setup for Claude Code
    When ejecuto domain setup claude-code
    Then configura la integración con claude-code
    And agrega el memory entry point al archivo de config de Claude Code
    And el output confirma "setup complete for claude-code"
    And proporciona instrucciones de uso

  Scenario: Setup for OpenCode
    When ejecuto domain setup opencode
    Then configura AGENTS.md para que opencode use memoria
    And agrega el plugin de memoria a la configuración de opencode
    And el output confirma "setup complete for opencode"

  Scenario: Setup for Gemini CLI
    When ejecuto domain setup gemini-cli
    Then configura la integración con gemini-cli
    And agrega la herramienta de memoria al profile de gemini-cli
    And el output confirma "setup complete for gemini-cli"

  Scenario: Setup for Codex CLI
    When ejecuto domain setup codex
    Then configura la integración con codex CLI
    And agrega el memory hook a codex
    And el output confirma "setup complete for codex"

  Scenario: Setup for PI (Pearl AI)
    When ejecuto domain setup pi
    Then configura la integración con pi
    And agrega el plugin de memoria
    And el output confirma "setup complete for pi"

  Scenario: Setup unknown agent
    When ejecuto domain setup unknown-agent
    Then el comando retorna error "unknown agent: unknown-agent"
    And exit code es 1
    And muestra los agentes disponibles: claude-code, opencode, gemini-cli, codex, pi

  Scenario: Setup with existing config
    Given la configuración para "opencode" ya existe
    When ejecuto domain setup opencode
    Then el comando pregunta antes de sobreescribir (--force para saltar)
    Or no sobreescribe si no se usa --force
```

## Análisis breve

- **Qué pide realmente:** 4 comandos: `serve` inicia servidor HTTP (delega a REQ-05); `mcp` inicia servidor MCP en stdio (delega a REQ-04); `tui` lanza interfaz bubbletea (delega a REQ-06); `setup` configura agentes de IA para integrar memoria en su tooling.
- **Módulos sospechados:** `internal/cli/serve.go`, `internal/cli/mcp.go`, `internal/cli/tui.go`, `internal/cli/setup.go`; `internal/server/` para HTTP; `internal/mcp/` para MCP; `internal/tui/` para bubbletea; `internal/setup/` para configuraciones de agentes
- **Riesgos / dependencias:** Depende de REQ-04 (MCP), REQ-05 (HTTP API), REQ-06 (TUI), REQ-11 (agent plugins). Estos comandos arrancan procesos de larga duración (serve, mcp, tui) o modifican archivos de configuración externos (setup).
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Revisar codebase (grep) — proyecto greenfield
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** No existe Go code aún
- **Acción derivada:** Implementar CLI handlers con stubs/delegación a otras REQs
