# Proposal: HU-12.2-mcp-memory-tools

## Intención

Implementar las 12 tools MCP del sistema de memorias, equivalentes a las operaciones del paquete engram pero respaldadas por Postgres. Cada tool se registra en el servidor MCP (HU-12.1) y ejecuta operaciones CRUD contra la tabla `observations` con soporte de embeddings y búsqueda semántica.

## Scope

**Incluye:**

Las siguientes 12 tools MCP:

| Tool | Descripción | Args principales |
|---|---|---|
| `domain_mem_save` | Guardar memoria | title, content, type, scope, topic_key |
| `domain_mem_search` | Buscar memorias | query, limit, project, scope, type |
| `domain_mem_context` | Contexto del proyecto | project, scope |
| `domain_mem_timeline` | Historia cronológica | observation_id, before, after, project |
| `domain_mem_get_observation` | Obtener por ID | id |
| `domain_mem_delete` | Eliminar (soft) | id, hard_delete |
| `domain_mem_save_prompt` | Guardar prompt | content, session_id |
| `domain_mem_session_start` | Iniciar sesión | id, directory |
| `domain_mem_session_end` | Cerrar sesión | id, summary |
| `domain_mem_session_summary` | Guardar resumen | session_id, content |
| `domain_mem_capture_passive` | Captura pasiva | content, source, session_id |
| `domain_mem_stats` | Estadísticas | project |
| `domain_mem_suggest_topic_key` | Sugerir topic key | content, title, type |

**No incluye:**
- Embeddings generation (depende de REQ-06-llm-embeddings)
- Interfaz de usuario para gestionar memorias
- Sincronización con engram local
- Export/import de memorias

## Enfoque técnico

1. Cada tool es un handler en `internal/mcp/tools/memory/` con naming `tool_mem_save.go`
2. Handler firma: `func(ctx context.Context, args map[string]interface{}) (*mcp.CallToolResult, error)`
3. Cada handler llama a `internal/service/memory.Service` que encapsula la lógica de negocio
4. `MemoryService` usa `internal/db` para operaciones Postgres
5. `domain_mem_search` usa pgvector para búsqueda semántica (embedding + cosine distance)
6. `domain_mem_suggest_topic_key` usa extracción de keywords vía LLM o heurística
7. `domain_mem_delete` hace soft delete con `deleted_at` timestamp
8. Las tools se registran en `cmd/domain-mcp/main.go` vía `server.RegisterTool()`
9. Los inputSchema se definen como structs con tags JSON y se convierten a JSON Schema

## Riesgos

- **Embedding no disponible:** Si el servicio de embeddings falla, domain_mem_search no funcional. Mitigación: fallback a búsqueda full-text con content_tsv.
- **Domain de sesión:** domain_mem_session_start/end requieren persistencia de sesiones. Mitigación: tabla sessions en Postgres.
- **Large content:** Domain muy grande puede exceder límites MCP. Mitigación: truncar a 10k chars, devolver resumen.
- **Concurrencia:** Muchas tools simultáneas pueden saturar DB. Mitigación: connection pool configurable.

## Testing

- Unit: cada tool handler con MemoryService mock
- Unit: validación de argumentos (requeridos, tipos)
- Integration: herramientas reales contra Postgres de test
- Integration: domain_mem_search con embeddings reales
- Integration: domain_mem_timeline con datos históricos
