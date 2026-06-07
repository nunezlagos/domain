# Proposal: HU-05.2-http-observations

## Intención

Exponer los 6 endpoints REST para el ciclo de vida completo de observaciones: crear, listar recientes con filtros, obtener por ID, actualizar, eliminar (soft/hard), y captura pasiva. El POST /observations debe detectar conflict candidates vía normalized_hash y devolver advertencia sin bloquear la creación.

## Scope

**Incluye:**
- `POST /observations` — crear observación con detección de conflict candidates (similarity check vía normalized_hash)
- `GET /observations/recent` — listar recientes con filtros opcionales: `?limit=&project=&type=&scope=`
- `GET /observations/{id}` — obtener por ID numérico
- `PATCH /observations/{id}` — actualizar campos (title, content, revision_count, etc.)
- `DELETE /observations/{id}` — soft delete (set deleted_at) por defecto; `?hard=true` para DELETE físico
- `POST /observations/passive` — crear observación sin conflict detection, modo silencioso
- JSON consistente en todas las respuestas

**No incluye:**
- Búsqueda full-text (HU-05.3)
- Timeline (HU-05.3)
- Autenticación (HU-05.9)
- Merge de proyectos (HU-05.7)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| POST /observations conflict detection | Buscar por `normalized_hash` después de insertar; si existe, incluir `conflict_candidate` en response |
| Soft delete | SET `deleted_at = datetime('now')`; GET/PATCH filtran `WHERE deleted_at IS NULL` |
| Hard delete | DELETE físico; requiere query param `?hard=true` |
| Passive capture | Mismo INSERT que POST pero sin conflict detection y sin validación de título |
| PATCH merge | Leer observation actual, hacer merge con campos del body, UPDATE |
| Filtros recent | Query params opcionales: limit (default 20, max 100), project, type, scope |

```go
type CreateObservationRequest struct {
    SessionID string `json:"session_id"`
    Title     string `json:"title,omitempty"`
    Content   string `json:"content"`
    Type      string `json:"type,omitempty"`
    Project   string `json:"project,omitempty"`
    Scope     string `json:"scope,omitempty"`
    ToolName  string `json:"tool_name,omitempty"`
    TopicKey  string `json:"topic_key,omitempty"`
}

type ConflictCandidate struct {
    ExistingID     int     `json:"existing_id"`
    SimilarityScore float64 `json:"similarity_score"`
    ExistingTitle  string  `json:"existing_title"`
}
```

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Conflict detection agrega latencia a POST | Media | Hacer búsqueda de normalized_hash en mismo INSERT (returning) o query secundaria rápida |
| Soft delete filtra en todas las queries | Alta | Todas las queries GET/PATCH incluyen `WHERE deleted_at IS NULL`; olvidarlo es bug |
| Hard delete no chequea permisos | Baja | Hard delete requiere ENGRAM_HTTP_TOKEN (HU-05.9) |
| PATCH parcial con campos nulos | Media | Usar `*string` en request para distinguir "no enviado" de "vacío" |

## Testing

- **Create:** POST /observations completo → 201 + ID
- **Create minimal:** POST solo session_id + content → 201
- **Create 400:** POST sin session_id ni content → 400
- **Conflict candidate:** POST dos observaciones similares → segunda tiene `conflict_candidate`
- **Recent:** GET /observations/recent → array DESC
- **Recent filtered:** GET /observations/recent?project=X&type=Y → filtrado
- **Get by ID:** GET /observations/{id} → 200
- **Get 404:** GET /observations/9999 → 404
- **Patch:** PATCH /observations/{id} → 200, campos actualizados
- **Patch 404:** PATCH /observations/9999 → 404
- **Soft delete:** DELETE → 204, GET posterior → 404
- **Hard delete:** DELETE?hard=true → 204
- **Passive capture:** POST /observations/passive → 201, sin conflict detection
- **Sabotaje:** Comentar `WHERE deleted_at IS NULL` en GET → muestra soft-deleted → test cae → restaurar
