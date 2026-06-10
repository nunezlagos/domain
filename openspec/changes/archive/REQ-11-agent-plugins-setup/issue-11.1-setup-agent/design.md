# Design: issue-11.1-setup-agent

## Decisión arquitectónica

### Agent definitions

```go
type Agent struct {
    Name       string
    BinaryName string          // para LookPath
    Configs    []AgentConfig   // archivos a crear/modificar
}

type AgentConfig struct {
    Path     string // ruta relativa al project root
    Template string // contenido template
    Mode     string // "create", "append_section", "update_section"
    Marker   string // marker para idempotencia
}

var supportedAgents = []Agent{
    {
        Name: "claude-code", BinaryName: "claude",
        Configs: []AgentConfig{
            {Path: "CLAUDE.md", Mode: "create", Template: claudeTemplate},
        },
    },
    {
        Name: "opencode", BinaryName: "opencode",
        Configs: []AgentConfig{
            {Path: "AGENTS.md", Mode: "append_section", Template: opencodeTemplate, Marker: "engram-memory-protocol"},
        },
    },
    {
        Name: "gemini-cli", BinaryName: "gemini",
        Configs: []AgentConfig{
            {Path: ".gemini/manifest.json", Mode: "create", Template: geminiTemplate},
        },
    },
    {
        Name: "codex", BinaryName: "codex",
        Configs: []AgentConfig{
            {Path: ".codex/setup.json", Mode: "create", Template: codexTemplate},
        },
    },
    {
        Name: "pi", BinaryName: "pi",
        Configs: []AgentConfig{
            {Path: "pi.config.json", Mode: "update_section", Template: piTemplate, Marker: "engram"},
        },
    },
    {
        Name: "vs-code", BinaryName: "code",
        Configs: []AgentConfig{
            {Path: ".vscode/settings.json", Mode: "update_section", Template: vscodeTemplate, Marker: "engram.memory"},
        },
    },
}
```

### Setup command

```
engram setup [flags]

Flags:
  --agent <name>    : specific agent to configure
  --detect          : auto-detect installed agents
  --dry-run         : show what would be done without writing
  --project <path>  : project directory (default: cwd)
```

### Detection

```go
func detectAgents() []string {
    var detected []string
    for _, agent := range supportedAgents {
        if _, err := exec.LookPath(agent.BinaryName); err == nil {
            detected = append(detected, agent.Name)
        }
    }
    // Also check for config files
    if _, err := os.Stat("CLAUDE.md"); err == nil {
        detected = addIfNotExists(detected, "claude-code")
    }
    return detected
}
```

### Template examples

```go
const claudeTemplate = `# Memory Protocol (engram)

This project uses engram for persistent memory across sessions.

## When to save
- After completing a task or subtask
- When you learn something about the project
- Before closing a session

## Commands
- \`engram save "title" "content"\` — save an observation
- \`engram search "query"\` — search memories
`

const opencodeTemplate = `
## Memory: engram

This project uses engram for persistent memory.
Refer to the engram CLI for saving and searching memories.
`
```

### Writer

```go
type FileWriter struct {
    DryRun bool
    Files  []FileAction
}

type FileAction struct {
    Path    string `json:"path"`
    Action  string `json:"action"` // created, modified, skipped
    Content string `json:"-"`
}

func (w *FileWriter) Write(path string, content string, mode string, marker string) error {
    action := FileAction{Path: path}

    switch mode {
    case "create":
        if _, err := os.Stat(path); err == nil {
            action.Action = "skipped (exists)"
            w.Files = append(w.Files, action)
            return nil
        }
        if !w.DryRun {
            os.WriteFile(path, []byte(content), 0644)
        }
        action.Action = "created"

    case "append_section":
        existing, _ := os.ReadFile(path)
        if strings.Contains(string(existing), marker) {
            action.Action = "skipped (already configured)"
            w.Files = append(w.Files, action)
            return nil
        }
        if !w.DryRun {
            f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
            f.WriteString(content)
            f.Close()
        }
        action.Action = "appended"
    }

    w.Files = append(w.Files, action)
    return nil
}
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Plugin system en cada agente | Cada agente tiene su propio mecanismo; no hay API común |
| Homebrew-style formula | No es necesario instalar los agentes; solo configurar |
| Wizard interactivo | Flags son suficientes; --detect hace el trabajo |

## TDD plan

1. **Red:** Setup crea CLAUDE.md para claude-code → falla
2. **Green:** Implement FileWriter + claude template → pasa
3. **Red:** Dry-run no escribe archivos → falla
4. **Green:** Implement DryRun check → pasa
5. **Red:** Detection encuentra claude en PATH → falla
6. **Green:** Implement detectAgents → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Templates desactualizados cuando agentes cambien | Versionar templates; setup --update refresh |
| Overwrite de config existente sin querer | Siempre append o usar markers; nunca overwrite completo |
| Permisos insuficientes para escribir | Error claro; sugerir sudo o cambiar directorio |
