# Tasks: issue-03.4-daemon-commands

## Backend

- [ ] **B1: Implementar serve command handler**
      ```
      internal/cli/serve.go
      ```
      - `serveCmd` con arg opcional `[port]` (default 3000)
      - Flag `--http-token` (string, default "")
      - Validar que el puerto esté disponible: `net.Listen("tcp", ":port")` y cerrar
      - Output: "Serving HTTP API on :{port}"
      - Delegar a `internal/server.Start(port, token)` que por ahora es stub
      - Manejar SIGINT/SIGTERM para graceful shutdown (delegado al server)

- [ ] **B2: Implementar server stub**
      ```
      internal/server/server.go
      ```
      ```go
      func Start(port int, token string) error {
          return fmt.Errorf("HTTP server not available until REQ-05-http-api")
      }
      // Placeholder para futura implementación:
      // mux := http.NewServeMux()
      // mux.HandleFunc("/health", healthHandler)
      // mux.Handle("/api/", apiMiddleware(token))
      // return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
      ```

- [ ] **B3: Implementar MCP command handler**
      ```
      internal/cli/mcp.go
      ```
      - `mcpCmd` con flags `--tools` (default "default"), `--project`
      - Output: "MCP server ready (profile: {tools})" en stderr (stdout es para protocolo MCP)
      - Delegar a `internal/mcp.Start(tools, project)` que es stub
      - MCP protocolo corre sobre stdin/stdout

- [ ] **B4: Implementar MCP server stub**
      ```
      internal/mcp/mcp.go
      ```
      ```go
      func Start(toolsProfile string, project string) error {
          return fmt.Errorf("MCP server not available until REQ-04-mcp-server")
      }
      // Placeholder:
      // scanner := bufio.NewScanner(os.Stdin)
      // for scanner.Scan() {
      //     line := scanner.Text()
      //     // JSON-RPC parse + dispatch
      // }
      ```

- [ ] **B5: Implementar TUI command handler**
      ```
      internal/cli/tui.go
      ```
      - `tuiCmd` sin flags
      - Verificar `isatty.IsTerminal(os.Stdout.Fd())` — si no, error
      - Verificar tamaño de terminal (min 80x24) — si no, error
      - Output: "Starting TUI..." (breve, antes de que bubbletea tome control)
      - Delegar a `internal/tui.Start()` que es stub
      - Dependencia: `github.com/mattn/go-isatty` y `golang.org/x/term`

- [ ] **B6: Implementar TUI stub**
      ```
      internal/tui/tui.go
      ```
      ```go
      func Start() error {
          return fmt.Errorf("TUI not available until REQ-06-tui-terminal-ui")
      }
      // Placeholder:
      // p := tea.NewProgram(initialModel())
      // return p.Start()
      ```

- [ ] **B7: Implementar setup command handler**
      ```
      internal/cli/setup.go
      ```
      - `setupCmd` con arg obligatorio `<agent>` y flag `--force`
      - ValidArgs: claude-code, opencode, gemini-cli, codex, pi
      - Switch por agente, llamando a función específica
      - Si agente desconocido: error + lista de válidos
      - Cada función setup: verificar si config existe, preguntar (si no --force), escribir

- [ ] **B8: Implementar setupClaudeCode**
      ```go
      func setupClaudeCode(cmd *cobra.Command, force bool) error {
          configDir := filepath.Join(os.Getenv("HOME"), ".claude")
          configPath := filepath.Join(configDir, "claude_code_memory.json")
          os.MkdirAll(configDir, 0755)
          
          if fileExists(configPath) && !force {
              confirmed := confirm(cmd, "Config already exists. Overwrite?")
              if !confirmed { return nil }
          }
          
          config := map[string]any{
              "memory_provider": "Domain",
              "endpoint":        "http://localhost:3000",
              "auto_save":       true,
              "commands": map[string]string{
                  "save":    "memoria save \"{{title}}\" \"{{content}}\" --type {{type}}",
                  "search":  "memoria search \"{{query}}\"",
                  "context": "memoria context",
              },
          }
          return writeJSON(configPath, config)
      }
      ```

- [ ] **B9: Implementar setupOpenCode**
      ```go
      func setupOpenCode(cmd *cobra.Command, force bool) error {
          agentsPath := filepath.Join(os.Getenv("HOME"), ".config", "opencode", "AGENTS.md")
          os.MkdirAll(filepath.Dir(agentsPath), 0755)
          
          if fileExists(agentsPath) && !force {
              confirmed := confirm(cmd, "AGENTS.md already exists. Append memory config?")
              if !confirmed { return nil }
          }
          
          content := `
      ## Domain — Persistent Memory

      Usa estos comandos para mantener memoria persistente entre sesiones:

      - \`memoria save "título" "detalle" --type fix --scope project\` — guardar decisión
      - \`memoria search "tema"\` — buscar en memoria
      - \`memoria context\` — ver contexto del proyecto
      - \`memoria stats\` — estadísticas de memoria

      Más información: https://github.com/nunezlagos/memoria
      `
          return appendToFile(agentsPath, content)
      }
      ```

- [ ] **B10: Implementar setupGeminiCLI**
      ```go
      func setupGeminiCLI(cmd *cobra.Command, force bool) error {
          toolsDir := filepath.Join(os.Getenv("HOME"), ".gemini", "tools")
          configPath := filepath.Join(toolsDir, "memoria.json")
          os.MkdirAll(toolsDir, 0755)
          
          if fileExists(configPath) && !force {
              confirmed := confirm(cmd, "Tool config exists. Overwrite?")
              if !confirmed { return nil }
          }
          
          toolDef := map[string]any{
              "name":        "Domain",
              "description": "Persistent memory store",
              "commands": map[string]any{
                  "save": map[string]any{
                      "command": "memoria save",
                      "args":    []string{"--type", "fix", "--scope", "project"},
                  },
                  "search": map[string]any{
                      "command": "memoria search",
                  },
              },
          }
          return writeJSON(configPath, toolDef)
      }
      ```

- [ ] **B11: Implementar setupCodex**
      ```go
      func setupCodex(cmd *cobra.Command, force bool) error {
          hooksDir := filepath.Join(os.Getenv("HOME"), ".codex", "hooks")
          hookPath := filepath.Join(hooksDir, "memoria.sh")
          os.MkdirAll(hooksDir, 0755)
          
          if fileExists(hookPath) && !force {
              confirmed := confirm(cmd, "Hook exists. Overwrite?")
              if !confirmed { return nil }
          }
          
          script := `#!/bin/bash
      # Codex memory hook - logs session context to memoria
      memoria save "codex: $CODEX_SESSION" "Working on $CODEX_TASK" --type general --scope project
      `
          return writeFile(hookPath, script)
      }
      ```

- [ ] **B12: Implementar setupPI**
      ```go
      func setupPI(cmd *cobra.Command, force bool) error {
          pluginsDir := filepath.Join(os.Getenv("HOME"), ".pi", "plugins")
          configPath := filepath.Join(pluginsDir, "memoria.json")
          os.MkdirAll(pluginsDir, 0755)
          
          if fileExists(configPath) && !force {
              confirmed := confirm(cmd, "Plugin config exists. Overwrite?")
              if !confirmed { return nil }
          }
          
          config := map[string]any{
              "name":        "Domain",
              "version":     "1.0",
              "description": "PI memory plugin using memoria",
              "hooks": map[string]string{
                  "on_session_end": "memoria save \"session: {{id}}\" \"{{summary}}\" --type session",
                  "on_decision":    "memoria save \"decision: {{title}}\" \"{{detail}}\" --type decision",
              },
          }
          return writeJSON(configPath, config)
      }
      ```

- [ ] **B13: Implementar helpers de setup**
      ```go
      // internal/cli/setup_helpers.go
      func fileExists(path string) bool { /* os.Stat */ }
      func writeJSON(path string, data any) error { /* json.MarshalIndent + os.WriteFile */ }
      func writeFile(path string, content string) error { /* os.WriteFile */ }
      func appendToFile(path string, content string) error { /* os.OpenFile(O_APPEND|O_CREATE) */ }
      func confirm(cmd *cobra.Command, msg string) bool {
          // leer respuesta sí/no de stdin
          cmd.Print(msg + " [y/N] ")
          var response string
          fmt.Scanln(&response)
          return strings.ToLower(response) == "y"
      }
      ```

- [ ] **B14: Registrar subcomandos en rootCmd**
      ```go
      func init() {
          // existing commands...
          rootCmd.AddCommand(serveCmd, mcpCmd, tuiCmd, setupCmd)
      }
      ```

## Frontend

- [ ] N/A — CLI tool (TUI es otro comando, no frontend web)

## Tests

- [ ] **T1: TestServeDefault** — serve sin args, verifica mensaje con puerto 3000
- [ ] **T2: TestServeCustomPort** — serve 8080, verifica puerto en mensaje
- [ ] **T3: TestServeWithToken** — serve --http-token, verifica flag parseado
- [ ] **T4: TestServePortInUse** — mock net.Listen error, verifica error "already in use"
- [ ] **T5: TestMCPDefault** — mcp, verifica output "MCP server ready"
- [ ] **T6: TestMCPToolsFlag** — mcp --tools minimal, verifica perfil en output
- [ ] **T7: TestMCPProjectFlag** — mcp --project memoria, verifica flag
- [ ] **T8: TestTUINotTerminal** — stdout pipe, verifica error "not a terminal"
- [ ] **T9: TestTUISmallTerminal** — terminal size mock <80x24, verifica error
- [ ] **T10: TestTUIValidTerminal** — terminal mock válida, verifica "Starting TUI"
- [ ] **T11: TestSetupClaudeCode** — setup claude-code, verifica archivo creado
- [ ] **T12: TestSetupOpenCode** — setup opencode, verifica AGENTS.md modificado
- [ ] **T13: TestSetupGeminiCLI** — setup gemini-cli, verifica tool config
- [ ] **T14: TestSetupCodex** — setup codex, verifica hook script
- [ ] **T15: TestSetupPI** — setup pi, verifica plugin config
- [ ] **T16: TestSetupUnknownAgent** — setup invalid-agent, verifica error + lista
- [ ] **T17: TestSetupForceFlag** — setup --force, sobreescribe sin confirmación
- [ ] **T18: TestSetupNoForceExisting** — setup sin --force, config exists → pregunta
- [ ] **T19: Sabotaje** — setup escribe en ruta sin permiso → error graceful
- [ ] **T20: Sabotaje** — serve con puerto negativo → error de parsing

## Cierre

- [ ] `go build ./cmd/domain` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/cli/... -v -count=1` — suite completa verde
- [ ] `./memoria serve --help` muestra flags correctamente
- [ ] `./memoria mcp --help` muestra flags correctamente
- [ ] `./memoria tui --help` muestra ayuda
- [ ] `./domain setup --help` muestra lista de agentes
- [ ] `./domain setup opencode` escribe en AGENTS.md
- [ ] Commit: `feat: implement daemon CLI commands serve/mcp/tui/setup`
