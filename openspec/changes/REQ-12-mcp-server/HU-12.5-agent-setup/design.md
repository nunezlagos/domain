# Design: HU-12.5-agent-setup

## Decisión arquitectónica

**Arquitectura de detectores + generadores + template engine.**

Cada agente soportado tiene un detector y un generador. El comando `domain setup` itera los detectores, muestra los encontrados, y ejecuta los generadores para los seleccionados.

```
domain setup [agent] [flags]
        │
        ▼
┌──────────────────┐
│   SetupCommand   │  cobra.Command
│   (cmd/domain/)  │
└──────┬───────────┘
       │
       ▼
┌────────────────────────────────────┐
│         SetupService               │
│  internal/setup/service.go         │
│                                    │
│  1. Detectar agentes instalados    │
│  2. Elegir agente objetivo         │
│  3. Generar config                 │
│  4. Generar .ai/directives.md      │
│  5. Aplicar (o dry-run)            │
└────────────────────────────────────┘
       │
       ├──────────────────┐
       ▼                  ▼
┌──────────────┐  ┌────────────────┐
│  AgentDetector │  │ ConfigGenerator│
│  interface     │  │ interface      │
├──────────────┤  ├────────────────┤
│ Detect()      │  │ Generate()     │
│   → []Agent   │  │   → Config     │
└──────────────┘  └────────────────┘
       │                  │
       ├── ClaudeDetector ├── ClaudeGenerator
       ├── OpenCodeDetec. ├── OpenCodeGenerator
       ├── CodexDetector  ├── CodexGenerator
       └── ClineDetector  └── ClineGenerator
```

**Rutas de configuración por agente:**

| Agent | Linux | macOS |
|-------|-------|-------|
| Claude Code | `~/.config/Claude/claude_desktop_config.json` | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| OpenCode | `.opencode/mcp.json` (project) | same |
| Codex | `~/.codex/config.json` | `~/Library/Application Support/Codex/config.json` |
| Cline | `~/.config/Cline/mcp_settings.json` | `~/Library/Application Support/Cline/mcp_settings.json` |

**Estructura de `.ai/directives.md`:**

```markdown
# Domain AI Directives

Eres un agente potenciado por Domain, una plataforma de agentes con
memoria persistente, skills reutilizables, flujos automatizados y SDD.

## Tools MCP de Domain (prefijo `domain_`)

Domain expone tools MCP con prefijo `domain_` para evitar conflictos.
Usalas como alternativa a tools nativas del agente cuando necesites:

| Para esto | Usá esta tool | En vez de |
|-----------|--------------|-----------|
| Guardar memoria | `domain_mem_save` | tool nativa |
| Buscar memoria | `domain_mem_search` | tool nativa |
| Ejecutar skill | `domain_skill_execute` | tool nativa |
| Crear flow | `domain_flow_create` | tool nativa |
| Ejecutar agente | `domain_agent_run` | - |

## CLI

Todos los comandos de Domain comienzan con `domain`:
- `domain memory save --title "..." --content "..." --type fix`
- `domain flow run --flow-id fl_xxx`
- `domain setup status`

## Arquitectura

Domain usa Postgres como backend. La memoria se persiste en la nube.
```
```

**Safe files pattern:** Lista de globs que el setup jamás modifica:
- `.env`, `.env.*`
- `*.pem`, `*.key`, `*.cer`
- `credentials.*`, `credentials.json`
- `.git/`, `.gitconfig`
- `id_rsa`, `id_ed25519`
- `*.local.*`

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|-------------|-------------------|
| Plugin del agente (extension API) | Cada agente tiene API de plugins diferente; MCP es el estándar común |
| Solo documentación manual ("agregá esto a tu config") | Mala UX, propenso a errores, la gente no lo hace |
| Setup vía web con copy-paste | Más seguro pero agregar fricción innecesaria; CLI es más directo |
| Overwrite total de config del agente | Rompe configs existentes con otros MCP servers; merge es obligatorio |
| Sin .ai/ folder (solo config MCP) | El agente necesita directivas para SABER que debe usar Domain; la config MCP sola no alcanza |

## TDD plan

1. **Red**: Escribir test que detecta Claude Code instalado (con mock de filesystem)
2. **Green**: Implementar `ClaudeDetector.Detect()` que busca en paths conocidos
3. **Refactor**: Extraer interface `AgentDetector`, agregar los otros detectores
4. **Red**: Test de `ClaudeGenerator.Generate()` produce JSON esperado
5. **Green**: Implementar generación de config con merge de MCP servers existentes
6. **Red**: Test de directivas genera contenido esperado
7. **Green**: Implementar template de `.ai/directives.md`
8. **Red**: Test de safe files — setup no modifica `.env`
9. **Green**: Implementar safe files protection con pattern matching
10. **Red**: Test de dry-run — no aplica cambios
11. **Green**: Implementar dry-run mode
12. **Sabotaje**: JSON corrupto en config existente → error parsing claro

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Config del agente en ubicación no estándar | Flag `--config-path` para override manual |
| Usuario tiene múltiples proyectos | MCP server global (una vez), .ai/ folder local (por proyecto) |
| Template de directivas se desactualiza | Versionado del template, `domain setup upgrade` para regenerar |
| Permisos denegados al escribir | Error claro + instrucciones de fix |
