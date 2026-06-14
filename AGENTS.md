# Domain — Project AGENTS.md

Override del `~/.config/opencode/AGENTS.md` global.

> **IMPORTANTE:** Este proyecto se construye 100% con agentes IA dirigidos por humanos. Antes de tocar nada, leé `.claude/rules/ai-generation.md` para entender el workflow.

## Stack
- **Lenguaje:** Go 1.22+
- **DB:** Postgres 15+ con pgvector, tsvector (vía pgx v5)
- **CLI:** Cobra + Viper
- **MCP:** mark3labs/mcp-go
- **Migraciones:** golang-migrate
- **Tests:** testing + testify + testcontainers-go
- **Logging:** slog

## SDD First
Todo cambio requiere HU. Las HUs están en `openspec/changes/`. Si no existe la HU, no se implementa.

## No tocar
- `openspec/` — documentación SDD, no código
- `.claude/` y `AGENTS.md` — config del agente
- Archivos archivados en `openspec/changes/archive/`

## Recordatorio
- Tools MCP con prefijo `domain_`
- CLI con `domain` (no `memoria`)
- Env vars con `DOMAIN_`
- NO agregar Co-Authored-By en commits
- NO hacer build después de cambios
