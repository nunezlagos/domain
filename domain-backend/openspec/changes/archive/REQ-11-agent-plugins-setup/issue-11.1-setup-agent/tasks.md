# Tasks: issue-11.1-setup-agent

## Backend

- [ ] **B1: Crear paquete `internal/setup/`**
      - `setup.go` — Setup command logic
      - `agents.go` — Agent definitions, detectAgents
      - `writer.go` — FileWriter con dry-run
      - `templates.go` — Template strings para cada agente

- [ ] **B2: Definir Agent struct y supportedAgents**
      - 6 agents: claude-code, opencode, gemini-cli, codex, pi, vs-code
      - Cada uno con BinaryName y Configs

- [ ] **B3: Implementar detectAgents**
      - exec.LookPath para cada binary
      - Check de config files existentes

- [ ] **B4: Implementar FileWriter**
      - Create mode: escribe si no existe
      - Append mode: agrega section con marker si no existe
      - Update mode: actualiza section entre markers
      - DryRun: simula y reporta
      - Report: lista de FileAction

- [ ] **B5: Crear templates**
      - CLAUDE.md template
      - AGENTS.md section template
      - .gemini/manifest.json template
      - .codex/setup.json template
      - pi.config.json section template
      - .vscode/settings.json section template

- [ ] **B6: Implementar `engram setup` CLI**
      - Flags: --agent, --detect, --dry-run, --project
      - Sin flags: --detect implícito, configura todos los detectados
      - Con --agent: configura específico
      - Con --detect: mustra detectados y configura

## Tests

- [ ] **T1: Setup claude-code crea CLAUDE.md**
- [ ] **T2: Setup opencode appends a AGENTS.md**
- [ ] **T3: Dry-run no escribe archivos**
- [ ] **T4: Idempotencia: segunda ejecución no duplica**
- [ ] **T5: detectAgents encuentra binarios en PATH**
- [ ] **T6: Reporte incluye paths y acciones**
- [ ] **T7: Setup con --detect configura todos los detectados**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/setup/... -v`
- [ ] Commit: `feat: agent setup for claude-code, opencode, gemini-cli, codex, pi, vs-code`
