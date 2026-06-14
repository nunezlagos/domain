# Proposal: issue-03.4-daemon-commands

## Intención

Proveer comandos para modos de operación de larga duración (serve, mcp, tui) y configuración de integración con agentes de IA (setup). Estos comandos transforman memoria de una CLI transaccional a un servicio/daemon. Cada uno delega a su respectiva REQ para la implementación del servidor/protocolo/UI, mientras que el CLI handler maneja flags, validación, y orquestación básica.

## Scope

**Incluye:**

- Comando `serve [port]` con flag `--http-token` — inicia servidor HTTP API (delega a REQ-05)
- Comando `mcp` con flags `--tools`, `--project` — inicia servidor MCP en stdio (delega a REQ-04)
- Comando `tui` — lanza interfaz bubbletea (delega a REQ-06)
- Comando `setup <agent>` — configura integración con agentes de IA (claude-code, opencode, gemini-cli, codex, pi)
- Stubs/delegación para comandos que dependen de REQs no implementadas
- Detección de terminal interactiva para tui
- Validación de puerto y disponibilidad para serve

**No incluye:**

- Implementación del servidor HTTP API (REQ-05)
- Implementación del protocolo MCP (REQ-04)
- Implementación de la TUI bubbletea (REQ-06)
- Implementación de plugins de agentes (REQ-11)
- Servicio systemd/launchd para auto-inicio

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Serve | Flag `--http-token`; default port 3000; validar disponibilidad con `net.Listen("tcp", ":port")` |
| MCP | STDIO mode via `os.Stdin`/`os.Stdout`; `--tools` profile name; `--project` filter |
| TUI | Verificar `isatty.IsTerminal(os.Stdout.Fd())` y tamaño de terminal antes de lanzar |
| Setup | Templates de configuración para cada agente; escribir archivos en ubicaciones estándar |
| Daemon lifecycle | `serve` y `mcp` son blocking; `tui` es blocking; todos responden a SIGINT/SIGTERM |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Serve/MCP/TUI no implementados en REQ respectivas | Alta | Stubs claros con "not available until REQ-X" |
| Setup modifica archivos existentes del usuario | Media | Preguntar antes de sobreescribir; flag `--force` |
| TUI no funciona en SSH/terminals limited | Media | Verificar terminal antes de iniciar |
| Serve deja el proceso en foreground | Baja | Por diseño; el usuario puede background con `&` o systemd |

## Testing

- **Serve:** test default port, custom port, token flag, port conflict
- **MCP:** test stdio handshake, tools profile, project filter (mock MCP server)
- **TUI:** test terminal detection (mock isatty), terminal size validation, graceful exit
- **Setup:** test cada agente, unknown agent, force flag, config file content verification
