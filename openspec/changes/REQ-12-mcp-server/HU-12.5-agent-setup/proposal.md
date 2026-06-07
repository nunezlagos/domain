# Proposal: HU-12.5-agent-setup

## Intención

Crear el comando `domain setup` que configura automáticamente los agentes de IA (Claude Code, OpenCode, Codex, Cline) para usar Domain como su servidor MCP principal. El comando es la puerta de entrada a la plataforma: sin setup, el agente no sabe que Domain existe.

## Scope

**Incluye:**
- Comando `domain setup` con auto-detección de agentes instalados
- Subcomandos: `domain setup claude-code`, `domain setup opencode`, `domain setup codex`, `domain setup cline`
- Subcomandos: `domain setup status`, `domain setup uninstall`
- Flag `--dry-run` para preview sin aplicar cambios
- Flag `--yes` para modo no interactivo (CI/CD)
- Detección de rutas de configuración por SO (Linux, macOS)
- Generación de `.ai/directives.md` con instrucciones para el agente
- Safe files protection: lista blanca de archivos que NUNCA se modifican
- Manejo de múltiples proyectos (MCP server global, .ai/ folder local)

**Excluye:**
- Web UI para setup (se hará en REQ-16)
- Configuración remota / cloud (el setup es local)
- Auto-actualización de la configuración (el agente debe reiniciarse)

## Enfoque técnico

1. **Detector de agentes**: Escanear rutas comunes de config (`~/.config/Claude/`, `.opencode/`, etc.) para detectar agentes instalados
2. **Config generators**: Para cada agente, un generador que produce el JSON de config con el MCP server de Domain
3. **Directives generator**: Template de `.ai/directives.md` con instrucciones para que el agente prefiera tools Domain
4. **Safe files**: Lista de patrones de archivos que nunca se modifican (`.env`, `*.pem`, `*.key`, `credentials.*`, `.git/`)
5. **Rollback**: Backup de config original antes de modificar, para poder hacer uninstall limpio

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Paths de config cambian entre versiones del agente | Medio | Mantener tabla de rutas por versión conocida, agregar override vía flag `--config-path` |
| Usuario tiene config personalizada que se sobreescribe | Alto | Siempre hacer merge (no overwrite), preservar otros MCP servers. Backup antes de modificar |
| Detección falla en OS no estándar | Bajo | Mensaje claro sugiriendo `--config-path` manual |
| .ai/directives.md obsoleto si cambian tool names | Medio | Template versionado, `domain setup upgrade` para actualizar |
| Permisos de escritura en config del agente | Medio | Error claro con instrucciones para arreglar permisos |

## Testing

- **Unitarios**: Cada detector de agente con rutas mockeadas, config generators con JSON expected
- **Integración**: Setup real en container con agente instalado, verificar que el MCP server aparece en tools/list
- **Regression**: Setup sobre config existente con otros MCP servers → no debe perderlos
- **Cross-platform**: Test en Linux y macOS (paths diferentes)
- **Sabotaje**: Corromper el JSON de config antes del setup → debe fallar con error parsing claro
