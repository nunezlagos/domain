# Proposal: HU-04.2-mcp-save-tools

## Intención

Exponer 5 herramientas MCP para escritura de memoria: `domain_mem_save`, `mem_update`, `domain_mem_delete`, `domain_mem_suggest_topic_key`, `domain_mem_save_prompt`. Estas son las tools que permiten a un agente persistir lo que aprende durante una sesión.

## Scope

**Incluye:**
- `domain_mem_save`: Crear observación con title, type, content, topic_key, scope, capture_prompt, project, session_id
- `mem_update`: Actualización parcial de observación existente por ID
- `domain_mem_delete`: Soft delete (flag) y hard delete (permanente) por ID
- `domain_mem_suggest_topic_key`: Generar slug/topic_key desde type + title usando heurísticas (lowercase, slugify, truncate a 80 chars)
- `domain_mem_save_prompt`: Guardar prompt de usuario con timestamp, alimentar ring buffer process-local (últimos 100)
- Conflict detection en `domain_mem_save`: si hay candidatos similares por título, devolverlos en `candidates[]` en lugar de rechazar
- Validación de tipos: solo valores conocidos del enum interno (decision, fix, pattern, context, artifact, session)

**Excluye:**
- Búsqueda (HU-04.3)
- Sesiones (HU-04.4)
- Admin (HU-04.5)
- Resolución de proyecto (HU-04.6)

## Enfoque técnico

1. **Archivo:** `internal/mcp/tools/save.go` con 5 funciones handler.
2. **Capa de negocio:** Llamadas a `engram.Engine` (o la interfaz de almacenamiento). Si el engine no existe aún, usar store simulado in-memory.
3. **Conflict detection:** Antes de insertar, hacer fuzzy match de título contra observaciones existentes. Si hay match >70% de similitud, incluir `candidates[]` en respuesta. No bloquear.
4. **Partial update:** `mem_update` acepta solo los campos a actualizar. Merge con el documento existente.
5. **Ring buffer:** Array circular de 100 slots en `internal/mcp/context.go`, accesible desde `save_prompt`.
6. **Input validation:** Schemas JSON Schema estrictos en cada tool definition.
7. **Topic key heuristic:** `slugify(lowercase(type) + "-" + truncate(title, 50))` + dedup suffix si colisiona.

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Conflict detection lento en inserts grandes | Latencia | Umbral de 100ms, si excede saltar detección |
| Ring buffer consume memoria | Leak | Tamaño fijo 100, overwrite circular |
| Hard delete irreversible | Data loss | Confirmación requerida (campo `confirm: true`) |
| Topic key colisiona | Sobrescritura | Append sufijo numérico `-2`, `-3` |

## Testing

- **Unit:** Cada tool con store in-memory. Casos borde: tipos inválidos, IDs inexistentes, conflictos exactos vs. parciales, ring buffer overflow.
- **Integration:** Server real con subprocess, todas las tools llamadas secuencialmente.
- **Sabotaje:** Enviar `domain_mem_save` sin title → esperar error. Enviar `mem_update` con id string → esperar type error.
