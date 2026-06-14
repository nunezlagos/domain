# Proposal: issue-04.6-mcp-profiles-resolution

## Intención

Implementar el sistema de perfiles de tools (default vs. agent) y la resolución automática de proyecto que atraviesa todas las tools MCP. Cada tool recibe el proyecto resuelto de forma transparente via middleware o wrapper, con overrides explícitos cuando el cliente los provee.

## Scope

**Incluye:**
- Dos perfiles: `default` (19 tools completas), `agent` (14 tools, excluye doctor/judge/compare/merge_projects)
- Flag `--profile` / `MEM_MCP_PROFILE` env en el server startup
- Middleware de resolución de proyecto: cada request pasa por un wrapper que inyecta `project` y `project_source` al handler
- Write tools: resolución implícita desde cwd, override explícito via parámetro `project`
- Read tools: resolución implícita desde cwd, override explícito via parámetro `project`, bypass via `all_projects`
- Response envelope estándar: `{ project, project_source, project_path, result }`
- `ENGRAM_PROJECT` env var: override global para MCP (prevalece sobre cwd)

**Excluye:**
- Implementación de las tools individuales (HUs 04.2-04.5)
- Lógica de resolución de proyecto ya definida en issue-04.5 (se reusa `internal/project/resolver.go`)

## Enfoque técnico

1. **Perfiles:** Mapa `map[string][]string` que define qué tools incluye cada perfil. Al start, filtrar el registry según el perfil activo.
2. **Middleware pattern:** Función `func ProjectResolver(next ToolHandler) ToolHandler` que:
   - Si la tool recibe `project` explícito → usar ese
   - Si `all_projects=true` → pasar nil (sin filtro)
   - Sino → resolver de cwd via resolver chain
   - Inyectar `project`, `project_source`, `project_path` en el contexto del handler
3. **Response envelope:** Struct genérico `ToolResponse { Project, ProjectSource, ProjectPath, Result }` que envuelve el resultado específico de cada tool.
4. **ENGRAM_PROJECT:** Revisión al inicio de la resolución. Si está seteada, es el proyecto definitivo (source="env").
5. **Server flag:** `flag.String("profile", "default", "tool profile: default|agent")` en `cmd/mcp.go`.

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Perfil cambia tools en runtime | Confusión | Perfil fijo al start. Si se necesita cambiar, reiniciar server. |
| Resolución de proyecto en cada request | Latencia | Cachear resultado de resolución por proceso (el cwd no cambia). |
| all_projects expone datos sensibles | Fuga de información | Solo accessible si el cliente MCP tiene permiso (el cliente MCP es de confianza). |

## Testing

- **Unit:** Test de profile filtering: default incluye 19, agent incluye 14. Test de middleware: project explícito pasa igual, implícito resuelve, all_projects bypass.
- **Integration:** Server con --profile agent → tools/list no incluye admin tools. Server con ENGRAM_PROJECT → todas las tools usan ese proyecto.
- **Sabotaje:** Profile name inválido → error al start. Project override con string vacío → ignorado.
