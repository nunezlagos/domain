# Tasks: issue-05.1-skill-definitions

## Backend

- [x] Crear migración SQL para tabla `skills` con todos los campos, constraints e índices
- [x] Implementar modelo `Skill` en Go (struct con tags sqlx)
- [x] Implementar validación de JSON Schema para `parameters` y `return_type`
- [x] Implementar handler POST /api/skills con validación y slug check
- [x] Implementar handler GET /api/skills con filtros (type, project_id, tags)
- [x] Implementar handler GET /api/skills/:id
- [x] Implementar handler PATCH /api/skills/:id con regeneración de embedding
- [x] Implementar handler DELETE /api/skills/:id con verificación de dependencias
- [x] Implementar goroutine de generación de embedding post-create/update
- [x] Implementar paginación (cursor o limit/offset) en list endpoint

## Frontend

- [x] N/A (fase inicial, solo API)

## Tests

- [x] Test unitario: creación de cada tipo de skill (prompt, code, api, mcp_tool)
- [x] Test unitario: validación de slug duplicado (mismo y distinto proyecto)
- [x] Test unitario: validación de JSON Schema inválido
- [x] Test unitario: actualización incrementa versión
- [x] Test unitario: borrado con y sin dependencias
- [x] Test unitario: embedding NULL cuando provider falla
- [x] Test de integración: ciclo completo CRUD vía HTTP
- [x] Sabotaje: eliminar provider de embeddings → confirmar que POST /api/skills sigue funcionando

## Cierre

- [x] Verificación manual con curl/httpie
- [x] Suite verde
