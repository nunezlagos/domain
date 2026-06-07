# Proposal: HU-01.2-observation-crud

## Intención

Que el store layer de memoria exponga un API CRUD completo para observaciones: crear con detección de conflictos, leer por ID, actualizar campos, eliminar (soft y hard), y listar con filtros. Es la operativa base sobre la que se apoyan todas las HUs siguientes (search, dedup, topic-key upsert, export/import).

## Scope

**Incluye:**

- Tipo `Observation` con todos los campos del schema (`internal/store/types.go`)
- Función `AddObservation(db, obs, candidatesOut) error` con:
  - Validación de required fields (title + content)
  - Cálculo de `normalized_hash` (SHA-256 de `project+scope+type+title+normalized_content`)
  - Detección de conflictos: `FindCandidates` previo al insert
  - Captura best-effort de prompt si `capture_prompt=true`
  - Retorno de candidates vía parámetro out `*[]Candidate`
- Función `GetObservation(db, id) (Observation, error)`
  - Query por PK, error si no existe o si está soft-deleted (según flag)
- Función `UpdateObservation(db, id, updates) error`
  - Actualización parcial de campos: title, content, type, scope, topic_key
  - Recalcula `normalized_hash` si cambia title/content
  - Incrementa `revision_count`
  - Actualiza `updated_at`
- Función `DeleteObservation(db, id, hard bool) error`
  - `hard=false`: setea `deleted_at` (soft delete)
  - `hard=true`: `DELETE FROM observations WHERE id = ?`
  - Error si soft-delete sobre observación ya eliminada
- Función `RecentObservations(db, filter) ([]Observation, error)`
  - Filtros: project, scope, type, limit (default 50), offset
  - Excluye soft-deleted por defecto
  - Orden DESC por created_at
- Tests de integración para cada operación CRUD
- Sabotaje: romper FK reference → confirmar error → restaurar

**No incluye:**

- Búsqueda FTS5 (HU-01.3)
- Deduplicación automática (HU-01.4) — solo detección/candidates
- Topic-key upsert automático (HU-01.5)
- Prompt storage flow completo (HU-01.6) — solo el flag capture_prompt
- Export/import (HU-01.8)
- UI/CLI layer — es puramente store API

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Paquete | `internal/store` — archivo `observations.go` con CRUD, `types.go` con structs |
| Driver | `modernc.org/sqlite` vía `database/sql` (mismo stack que HU-01.1) |
| Hash | `crypto/sha256` de `project+scope+type+title+normalized_content`, hex-encoded |
| Normalización | Lowercase + trim + collapse whitespace + remove punctuation básico |
| Candidates | `SELECT ... WHERE normalized_hash = ? AND id != ?` + opcional `content LIKE %?%` |
| Conflict return | Slice `[]Candidate{ID, Title, Reason}` como parámetro out en AddObservation |
| Prompt capture | Variable global `CurrentPrompt string` en el package; si capture_prompt=true y no vacía, INSERT en user_prompts dentro de la misma tx |
| Soft delete | `UPDATE observations SET deleted_at = datetime('now') WHERE id = ? AND deleted_at IS NULL` |
| Hard delete | `DELETE FROM observations WHERE id = ?` |
| Recent | `SELECT ... FROM observations WHERE deleted_at IS NULL [...filters...] ORDER BY created_at DESC LIMIT ? OFFSET ?` |
| FTS5 sync | No hacer nada explícito — los triggers de HU-01.1 ya sincronizan automáticamente |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Candidates falsos positivos por hash demasiado estricto | Media | El hash es exacto (SHA-256); candidates se basa en normalized_hash exacto + LIKE en content como secondary signal |
| Prompt capture pierde prompts por race condition | Baja | El prompt se lee al momento de la tx; si CurrentPrompt cambia entre lectura e INSERT, se pierde — aceptable por ser best-effort |
| Soft-delete observation referenciada por memory_relations | Baja | No hay CASCADE; la app debe validar o el schema podría agregar ON DELETE SET NULL en el futuro |
| FTS5 no se actualiza en hard delete | Baja | Los triggers AFTER DELETE ya cubren este caso |
| concurrent writes causan SQLITE_BUSY | Baja | busy_timeout=5000 maneja esto; si persiste, el error se propaga al caller |

## Testing

- **Integración:** Cada CRUD operation prueba contra SQLite en memoria con schema migrado
- **AddObservation:** Creación con todos los campos, omisión de required fields retorna error, normalized_hash se computa correctamente, candidates se retornan cuando hay match
- **GetObservation:** Lectura por ID existente, lectura de ID inexistente retorna error
- **UpdateObservation:** Actualización individual y múltiple de campos, normalized_hash se recalcula, revision_count se incrementa
- **DeleteObservation:** Soft delete setea deleted_at, hard delete remueve fila, doble soft delete retorna error
- **RecentObservations:** Filtros combinados, límite, exclusión de soft-deleted
- **Sabotaje:** Romper FK reference en INSERT → confirmar error → restaurar FK → test pasa
