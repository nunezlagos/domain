# Tasks: issue-05.1-skill-definitions

## Backend

- [ ] Crear migración SQL para tabla `skills` con todos los campos, constraints e índices
- [ ] Implementar modelo `Skill` en Go (struct con tags sqlx)
- [ ] Implementar validación de JSON Schema para `parameters` y `return_type`
- [ ] Implementar handler POST /api/skills con validación y slug check
- [ ] Implementar handler GET /api/skills con filtros (type, project_id, tags)
- [ ] Implementar handler GET /api/skills/:id
- [ ] Implementar handler PATCH /api/skills/:id con regeneración de embedding
- [ ] Implementar handler DELETE /api/skills/:id con verificación de dependencias
- [ ] Implementar goroutine de generación de embedding post-create/update
- [ ] Implementar paginación (cursor o limit/offset) en list endpoint

## Frontend

- [ ] N/A (fase inicial, solo API)

## Tests

- [ ] Test unitario: creación de cada tipo de skill (prompt, code, api, mcp_tool)
- [ ] Test unitario: validación de slug duplicado (mismo y distinto proyecto)
- [ ] Test unitario: validación de JSON Schema inválido
- [ ] Test unitario: actualización incrementa versión
- [ ] Test unitario: borrado con y sin dependencias
- [ ] Test unitario: embedding NULL cuando provider falla
- [ ] Test de integración: ciclo completo CRUD vía HTTP
- [ ] Sabotaje: eliminar provider de embeddings → confirmar que POST /api/skills sigue funcionando

## Cierre

- [ ] Verificación manual con curl/httpie
- [ ] Suite verde
