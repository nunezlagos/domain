# Proposal: HU-04.3-mcp-search-tools

## Intención

Exponer 5 herramientas MCP de consulta y navegación de memoria: `domain_mem_search` (FTS5 full-text), `domain_mem_context` (snapshot reciente), `domain_mem_timeline` (vecindad cronológica), `domain_mem_get_observation` (por ID sin truncar), `domain_mem_stats` (dashboard del sistema). Incluye anotaciones de conflicto en resultados de búsqueda.

## Scope

**Incluye:**
- `domain_mem_search`: Búsqueda FTS5 con filtros por query, type, project, scope, limit. Modo `all_projects`. Resultados paginados (default 20, max 100). Anotaciones de conflicto.
- `domain_mem_context`: Últimas N observaciones (default 10) + sesión activa + metadata del proyecto.
- `domain_mem_timeline`: Vecindad cronológica: `before` + `after` alrededor de `observation_id`. Orden cronológico estricto.
- `domain_mem_get_observation`: Obtener observación completa por ID, sin truncamiento (a diferencia de search que limita contenido a 500 chars).
- `domain_mem_stats`: Conteos agregados: total_observations, total_sessions, total_prompts, unique_projects, fechas extremas, storage_size_bytes.

**Excluye:**
- Escritura (HU-04.2)
- Sesiones (HU-04.4)
- Admin (HU-04.5)
- Resolución de proyecto (HU-04.6)

## Enfoque técnico

1. **FTS5 engine:** Usar `mattn/go-sqlite3` con FTS5 habilitado. Tabla virtual FTS5 sobre observaciones. Query con sintaxis FTS5 (términos, frases, prefix).
2. **Conflict annotations:** JOIN con tabla de conflictos pendientes (REQ-10). Si el observation_id tiene conflictos abiertos, incluir `conflicts: [{id, status, similar_field}]`.
3. **Context:** `SELECT * FROM observations WHERE project=? ORDER BY created_at DESC LIMIT 10`, más sesión activa si existe.
4. **Timeline:** Dos queries: `WHERE id < ? ORDER BY id DESC LIMIT before` y `WHERE id > ? ORDER BY id ASC LIMIT after`. Unir y ordenar.
5. **Get observation:** `SELECT * FROM observations WHERE id=?`, sin límite de longitud.
6. **Stats:** Queries de agregación: COUNT, MIN, MAX, SUM de page_count o similar para storage.
7. **Paginación:** Offset/limit con default 20, max 100.

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| FTS5 no disponible en SQLite | Crash | Compilar con CGO_ENABLED=1 y tag `fts5`. CI verifica. |
| Query muy larga (>1KB) | Rechazo | Limitar query a 500 chars, sanitizar caracteres especiales FTS5 |
| Timeline con IDs no contiguos | Huecos | Usar created_at en lugar de id para el ordering |
| Stats lento con millones de rows | Timeout | Cachear stats por 30s, o usar COUNT(*) aproximado |

## Testing

- **Unit:** Store in-memory con FTS5 mockeado o SQLite :memory:. Tests de parseo de query FTS5, filtros, paginación.
- **Integration:** Server real, insertar 50 observaciones, buscar por términos, verificar rankings.
- **Sabotaje:** Query con caracteres FTS5 especiales (`"`, `*`, `-`) → no crash. Timeline con id que no existe → error claro.
