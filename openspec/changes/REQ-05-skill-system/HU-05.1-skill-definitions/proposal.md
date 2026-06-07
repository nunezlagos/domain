# Proposal: HU-05.1-skill-definitions

## Intención

Implementar el CRUD completo de skills reutilizables. Los skills son el bloque fundamental del sistema de skills: definiciones parametrizables que los agentes pueden invocar. Soportamos 4 tipos: prompt (template de texto con variables), code (código ejecutable), api (llamada HTTP REST), y mcp_tool (tool MCP estándar).

## Scope

**Incluye:**
- Modelo `Skill` con campos: id, name, slug, description, type, content, project_id, parameters (JSONB), return_type (JSONB), tags (TEXT[]), embedding (vector(1536)), version, created_at, updated_at
- Endpoints REST: POST/GET /api/skills, GET/PATCH/DELETE /api/skills/:id
- Validación de JSON Schema en `parameters` y `return_type`
- Generación de embedding al crear/actualizar (vía HU-06.5)
- Filtros en list: type, project_id, tags (array overlap)
- Slug único por proyecto
- Protección contra borrado con dependencias activas

**Excluye:**
- Búsqueda semántica (HU-05.2)
- Versionado completo (HU-05.3)
- Ejecución de skills (HU-05.5)

## Enfoque técnico

- Modelo SQL con GORM o sqlx. Columna `parameters` y `return_type` como JSONB. `tags` como TEXT[] con índice GIN.
- Handler REST estándar con validator de JSON Schema (library `santhosh-tekuri/jsonschema`).
- Slug único compuesto: `(project_id, slug)` unique constraint.
- Embedding se genera post-save llamando al embedding provider. Si falla, el skill se crea igual pero con embedding NULL (graceful degradation).
- Para soft-dependency check, consultar tabla `flow_steps` y `agent_skills` antes de DELETE.

## Riesgos

- Dependencia de embedding provider: si no hay LLM configurado, los embeddings serán NULL y la búsqueda semántica no funcionará. Mitigación: logging warning + campo embedding nullable.
- JSON Schema inválido: el usuario podría mandar schemas cíclicos o demasiado complejos. Mitigación: validar con tamaño máximo y profundidad máxima.
- Slug collision: la unique constraint compuesta maneja el caso edge.

## Testing

- **Unitarios:** Creación con datos válidos, creación con slug duplicado (mismo project y distinto project), validación de JSON Schema, update incrementa versión, delete con/sin dependencias.
- **Integración:** POST + GET list + GET by id + PATCH + DELETE, verificar embedding generado (mock del provider).
- **E2E:** Flujo completo crear → listar → obtener → actualizar → eliminar.
- **Sabotaje:** Romper el provider de embeddings → verificar que el skill se crea con embedding NULL y no hay panic.
