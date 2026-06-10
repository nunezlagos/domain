# Proposal: issue-11.1-setup-agent

## Intención

Implementar `engram setup` que configura automáticamente la integración de memoria con los principales CLIs de IA y editores. Detecta agentes instalados y escribe los archivos de configuración/plugin necesarios.

## Scope

**Incluye:**
- `engram setup [--agent <name>] [--detect] [--dry-run]`
- Soporte para: claude-code, opencode, gemini-cli, codex, pi, vs-code
- Escritura de CLAUDE.md, AGENTS.md, .vscode/settings.json, .codex/setup.json, pi.config.json, gemini manifest
- Detección automática de agente via PATH y archivos existentes
- Idempotencia (no duplicar secciones)
- Reporte de archivos creados/modificados

**No incluye:**
- Instalación de los agentes (asume que ya están instalados)
- Memory Protocol content (issue-11.2)
- Uninstall o cleanup

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Agent detection | `exec.LookPath("claude")`, `exec.LookPath("opencode")`, check de archivos existentes |
| Config writing | Template strings con placeholders; append o create según corresponda |
| Idempotencia | Marker comments `<!-- engram:start -->...<!-- engram:end -->` en archivos |
| Dry-run | Simular escritura en memoria (strings.Builder) y mostrar diff |
| Reporte | Lista de `FileAction{Path, Action, Content}` |

