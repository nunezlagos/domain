# Proposal: issue-05.4-http-prompts

## Intención

Exponer 4 endpoints para el manejo de prompts de usuario: guardar prompts asociados a sesiones, listar recientes, buscar por contenido via FTS5, y eliminar. Es el equivalente HTTP de `domain_mem_save_prompt` del MCP y CLI.

## Scope

**Incluye:**
- `POST /prompts` — guardar prompt con session_id (required), content (required), project (optional)
- `GET /prompts/recent` — listar prompts recientes con ?limit= (default 20)
- `GET /prompts/search?q=&project=` — buscar prompts via FTS5 prompts_fts
- `DELETE /prompts/{id}` — eliminar prompt por ID
- Respuestas JSON consistentes

**No incluye:**
- Actualización de prompts (no hay PATCH)
- GET individual por ID (no necesario para el caso de uso actual)
- Autenticación (issue-05.9)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| POST /prompts | Insert simple en user_prompts; triggers FTS5 sincronizan automáticamente |
| GET /prompts/recent | `SELECT ... FROM user_prompts ORDER BY created_at DESC LIMIT ?` |
| GET /prompts/search | FTS5 MATCH + JOIN user_prompts + WHERE project filter |
| DELETE | `DELETE FROM user_prompts WHERE id = ?` (físico, no hay soft delete para prompts) |
| Search reuso | Mismo sanitizer FTS5 que issue-05.3 |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| POST con session_id inválido | Media | Validar existencia de sesión; si no existe → 400 |
| FTS5 prompts_fts no configurado | Baja | Dependencia de issue-01.1; triggers deben existir |

## Testing

- **Save:** POST /prompts → 201 + ID
- **Save 400:** sin content o session_id → 400
- **Recent:** GET /prompts/recent → array DESC
- **Recent limit:** GET /prompts/recent?limit=3 → max 3
- **Search:** GET /prompts/search?q=hello → resultados FTS5
- **Search 400:** sin q → 400
- **Search project:** GET /prompts/search?q=hello&project=X → filtrado
- **Delete:** DELETE /prompts/{id} → 204
- **Delete 404:** DELETE /prompts/9999 → 404
