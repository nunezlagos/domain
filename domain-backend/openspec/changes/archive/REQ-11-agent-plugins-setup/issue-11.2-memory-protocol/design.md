# Design: issue-11.2-memory-protocol

## Decisión arquitectónica

### Document structure

```
# Memory Protocol v1.0.0
**Last updated:** 2026-06-07

## 1. WHEN_TO_SAVE

Save an observation when:
- **Task completion:** After finishing any non-trivial task
- **Project context:** When you learn something about the project structure
- **Decisions:** When an architectural or design decision is made
- **Before close:** Before session ends, save a summary
- **Errors:** When you discover a bug or workaround

### Example
```go
domain_mem_save("fix: handle null pointer in parser", "Root cause was...")
```

## 2. WHEN_TO_SEARCH

Search before:
- **Assumptions:** Before making claims about the codebase
- **References:** When you need to recall past work
- **Patterns:** When detecting recurring issues or solutions
- **Context switch:** When returning to a project after a break

### Example
```
domain_mem_search "null pointer parser"
```

## 3. Topic update rules

- Derive topic_key from the primary subject
- Reuse existing topic_key when content is related
- Create new topic_key when content is sufficiently distinct
- Use format: `area/component` or `concept` (lowercase, no spaces)

### Example
```go
// Reuse existing topic
domain_mem_save("fix parser crash", "...", topic="parser")

// New topic for distinct area
domain_mem_save("add user auth endpoint", "...", topic="auth")
```

## 4. Session close protocol

1. Call `domain_mem_save` with session summary as title
2. Format: "Session: {date} - {project} - {accomplished}"
3. Call `domain_mem_session_summary` with accomplished + next_steps
4. Session status is set to 'closed' automatically

### Example
```go
domain_mem_save("Session: 2026-06-07 - my-app - refactored parser",
    "Accomplished: fixed null pointer, added tests. Next: add error recovery")
```

## 5. Passive capture

Automatically capture from tool output:
- **Tool results:** stdout/stderr from commands
- **Errors:** Compilation errors, test failures
- **Patterns:** Successful command sequences
- **Configs:** Important configuration values discovered

### Rules
- Deduplicate: skip if similar content was recently captured
- Prioritize: errors > results > info
- Size limit: truncate content > 2000 characters

## 6. After compaction

After memory compaction (deduplication):
- Search for topics that may have been merged
- Update references in new observations
- Verify that consolidated topics still have relevant context

### Example
```go
// After compaction, re-search for affected topics
domain_mem_search("parser")
// Review results and add clarification if needed
```

## Changelog

- v1.0.0 (2026-06-07): Initial protocol definition
```

### Embedding in binary

```go
//go:embed MEMORY_PROTOCOL.md
var memoryProtocolMD string

func init() {
    // Register protocol command
}
```

### CLI command

```
engram protocol [--section <name>]
```

Without flags: prints full protocol. With `--section`: prints specific section.

### Integration with issue-11.1

The `engram setup --agent claude-code` command will write CLAUDE.md with:

```markdown
# Memory Protocol

This project uses engram for persistent memory.
See MEMORY_PROTOCOL.md for the full protocol.

## Quick reference
- Save after tasks, decisions, and before closing
- Search before making assumptions
- Use topic_key for organization
- Close sessions properly
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| Protocolo solo en web docs | Debe estar disponible offline; embed en binary es mejor |
| JSON/YAML estructurado | Markdown es más legible por humanos y agentes; parseo no necesario |
| Múltiples protocolos por agente | Un protocolo unificado es más simple; cada agente adapta via setup |

## TDD plan

No aplica testing tradicional (es documento). Verificar:
- `go build` con embed funciona
- `engram protocol` imprime el documento
- Referencia cruzada: setup incluye mención al protocolo

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Protocolo se desactualiza | Versionado semver; checklist en cada release para revisar protocolo |
| Agentes no siguen el protocolo | Es guía, no enforcement; futuras HUs pueden agregar validación |
| Documento muy largo | Secciones bien definidas; `--section` flag para consulta específica |
