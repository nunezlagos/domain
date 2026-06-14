# Design: issue-03.4-daemon-commands

## Decisión arquitectónica

### Serve command

```go
var serveCmd = &cobra.Command{
    Use:   "serve [port]",
    Short: "Start HTTP API server",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        port := 3000
        if len(args) > 0 {
            port, _ = strconv.Atoi(args[0])
        }
        httpToken, _ := cmd.Flags().GetString("http-token")

        addr := fmt.Sprintf(":%d", port)
        listener, err := net.Listen("tcp", addr)
        if err != nil {
            return fmt.Errorf("port %d already in use: %w", port, err)
        }
        listener.Close() // release, server will listen again

        cmd.Printf("Serving HTTP API on %s\n", addr)
        // delegar a internal/server.Start(addr, httpToken)
        return fmt.Errorf("serve not available until REQ-05-http-api")
    },
}
```

Flags: `--http-token`

### MCP command

```go
var mcpCmd = &cobra.Command{
    Use:   "mcp",
    Short: "Start MCP server (Model Context Protocol)",
    RunE: func(cmd *cobra.Command, args []string) error {
        tools, _ := cmd.Flags().GetString("tools")
        project, _ := cmd.Flags().GetString("project")

        if tools == "" { tools = "default" }

        cmd.Printf("MCP server ready (profile: %s)\n", tools)
        // delegar a internal/mcp.Start(tools, project)
        return fmt.Errorf("MCP server not available until REQ-04-mcp-server")
    },
}
```

Flags: `--tools` (profile name: default, minimal, full), `--project`

MCP corre en modo stdio. El proceso lee de stdin y escribe a stdout siguiendo el protocolo MCP (JSON-RPC). Esto permite integración directa con Claude Desktop, Cursor, etc.

### TUI command

```go
var tuiCmd = &cobra.Command{
    Use:   "tui",
    Short: "Launch terminal UI",
    RunE: func(cmd *cobra.Command, args []string) error {
        if !isatty.IsTerminal(os.Stdout.Fd()) {
            return fmt.Errorf("not a terminal")
        }

        width, height, err := terminal.GetSize(int(os.Stdout.Fd()))
        if err != nil || width < 80 || height < 24 {
            return fmt.Errorf("terminal too small (min 80x24, got %dx%d)", width, height)
        }

        cmd.Println("Starting TUI...")
        // delegar a internal/tui.Start()
        return fmt.Errorf("TUI not available until REQ-06-tui-terminal-ui")
    },
}
```

No flags. Verifica terminal interactiva y tamaño mínimo antes de lanzar.

### Setup command

```go
var setupCmd = &cobra.Command{
    Use:       "setup <agent>",
    Short:     "Configure memory integration for an AI coding agent",
    Args:      cobra.ExactArgs(1),
    ValidArgs: []string{"claude-code", "opencode", "gemini-cli", "codex", "pi"},
    RunE: func(cmd *cobra.Command, args []string) error {
        agent := args[0]
        force, _ := cmd.Flags().GetBool("force")

        switch agent {
        case "claude-code":
            return setupClaudeCode(cmd, force)
        case "opencode":
            return setupOpenCode(cmd, force)
        case "gemini-cli":
            return setupGeminiCLI(cmd, force)
        case "codex":
            return setupCodex(cmd, force)
        case "pi":
            return setupPI(cmd, force)
        default:
            return fmt.Errorf("unknown agent: %s (valid: claude-code, opencode, gemini-cli, codex, pi)", agent)
        }
    },
}
```

Flags: `--force` (skip confirmation on overwrite)

### Setup implementations

Cada setup escribe archivos de configuración específicos del agente:

```go
func setupClaudeCode(cmd *cobra.Command, force bool) error {
    // Claude Code usa ~/.claude/claude_code_memory.json
    configPath := filepath.Join(os.Getenv("HOME"), ".claude", "claude_code_memory.json")
    if fileExists(configPath) && !force {
        if !confirmOverwrite(cmd, configPath) { return nil }
    }
    config := map[string]any{
        "memory_provider": "Domain",
        "endpoint":        "http://localhost:3000",
        "auto_save":       true,
    }
    return writeJSON(configPath, config)
}

func setupOpenCode(cmd *cobra.Command, force bool) error {
    // OpenCode: agrega entrada a ~/.config/opencode/AGENTS.md
    agentsPath := filepath.Join(os.Getenv("HOME"), ".config", "opencode", "AGENTS.md")
    if fileExists(agentsPath) && !force {
        if !confirmOverwrite(cmd, agentsPath) { return nil }
    }
    content := `
## Domain (persistent memory)

Whenever you save a decision, fix, or pattern:
\`\`\`
memoria save "<title>" "<detail>" --type <type> --scope project
\`\`\`
To recall context:
\`\`\`
memoria search "<query>"
memoria context
\`\`\`
`
    return appendToFile(agentsPath, content)
}

func setupGeminiCLI(cmd *cobra.Command, force bool) error {
    // Gemini CLI: tool config
    configPath := filepath.Join(os.Getenv("HOME"), ".gemini", "tools", "memoria.json")
    os.MkdirAll(filepath.Dir(configPath), 0755)
    return writeJSON(configPath, geminiToolDefinition())
}

func setupCodex(cmd *cobra.Command, force bool) error {
    // Codex CLI: memory hook
    hooksDir := filepath.Join(os.Getenv("HOME"), ".codex", "hooks")
    os.MkdirAll(hooksDir, 0755)
    return writeFile(filepath.Join(hooksDir, "memoria.sh"), codexHookScript())
}

func setupPI(cmd *cobra.Command, force bool) error {
    // PI (Pearl AI): plugin config
    configPath := filepath.Join(os.Getenv("HOME"), ".pi", "plugins", "memoria.json")
    os.MkdirAll(filepath.Dir(configPath), 0755)
    return writeJSON(configPath, piPluginConfig())
}
```

### Signal handling

Serve, MCP y TUI son procesos de larga duración. Deben manejar señales:

```go
func handleShutdown(cancel context.CancelFunc) {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigCh
        cancel()
        // graceful shutdown
    }()
}
```

### Daemon process model

| Comando | Tipo | Comunicación | Detach |
|---------|------|-------------|--------|
| serve | HTTP server | TCP | No (foreground) |
| mcp | MCP server | STDIO | No (foreground) |
| tui | TUI app | Terminal | No (foreground) |
| setup | One-shot | CLI | N/A |

Ninguno daemoniza por sí mismo. El usuario puede usar `&`, `nohup`, o systemd según su preferencia.

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Daemonizacion automática (fork) | Complicado en Go; mejor dejar al OS/systemd |
| Config de serve en archivo | Flags son más explícitos para CLI; config file redunda |
| MCP modo TCP además de stdio | Modo stdio es el estándar MCP; TCP es para serve HTTP |
| Setup automático post-instalación | El usuario debe elegir qué agente configurar |
| TUI lanzada por defecto sin flag | CLI debe ser CLI por defecto; TUI es opt-in |

## TDD plan

1. **Red:** `TestServeCommand` — serve default port, verifica output → falla
2. **Green:** Serve handler con stub message → pasa
3. **Red:** `TestServeCustomPort` — serve 8080, verifica port en mensaje → falla
4. **Green:** Parse port arg → pasa
5. **Red:** `TestServePortInUse` — puerto ocupado, error → falla
6. **Green:** net.Listen test → pasa
7. **Red:** `TestMCPCommand` — mcp default, verifica output → falla
8. **Green:** MCP handler stub → pasa
9. **Red:** `TestMCPToolsProfile` — --tools minimal → verifica perfil → falla
10. **Green:** Parse tools flag → pasa
11. **Red:** `TestTUICommand` — tui en terminal, verifica "Starting TUI" → falla
12. **Green:** TUI handler + isatty mock → pasa
13. **Red:** `TestTUINotTerminal` — stdout no es terminal → error → falla
14. **Green:** isatty check → pasa
15. **Red:** `TestTUISmallTerminal` — terminal <80x24 → error → falla
16. **Green:** terminal size check → pasa
17. **Red:** `TestSetupClaudeCode` — setup claude-code → verifica archivo creado → falla
18. **Green:** Implementar setupClaudeCode → pasa
19. **Red:** `TestSetupOpenCode` / `TestSetupGeminiCLI` / `TestSetupCodex` / `TestSetupPI` → fallan
20. **Green:** Implementar cada setup → pasan
21. **Red:** `TestSetupUnknownAgent` → error → falla
22. **Green:** default case con error + lista de agentes → pasa
23. **Red:** `TestSetupForceFlag` — setup con --force sobreescribe sin confirmación → falla
24. **Green:** force flag skip confirm → pasa
25. **Sabotaje:** Setup escribe en ruta sin permiso → error graceful

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Serve/MCP/TUI stubs quedan permanentemente | Cada stub referencia REQ específica; son prioridades independientes |
| Setup corrompe config de agente existente | `--force` requerido para sobreescribir; si no hay force, warning y skip |
| MCP stdio blockea si no hay cliente | Timeout en handshake; el proceso termina si no hay connect en N segundos |
| TUI crashea en terminal no soportada | Verificación previa de terminal size + isatty |
| Serve sin --http-token es inseguro | Default: token requerido; solo /health es público |
