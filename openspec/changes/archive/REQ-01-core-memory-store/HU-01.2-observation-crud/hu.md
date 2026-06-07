# HU-01.2-observation-crud

**Origen:** `REQ-01-core-memory-store`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario del sistema de memoria  
**Quiero** crear, leer, actualizar y eliminar observaciones  
**Para** poder persistir y gestionar el conocimiento generado en sesiones de desarrollo

## Criterios de aceptación

```gherkin
Scenario: Create observation with all fields
  Given una sesión activa con ID "s1"
  When creo una observación con:
    | type       | "fix"              |
    | title      | "Bug en login"     |
    | content    | "El modal no cierra al hacer submit" |
    | tool_name  | "opencode"         |
    | project    | "Domain"          |
    | scope      | "project"          |
    | topic_key  | "auth"             |
    | session_id | "s1"               |
  Then la observación se crea con ID > 0
  And los campos se persisten exactamente como fueron enviados
  And la observación tiene created_at y updated_at no nulos
  And tiene normalized_hash calculado automáticamente
  And se indexa automáticamente en observations_fts

Scenario: Get observation by ID
  Given una observación existente con ID 42
  When consulto la observación por ID 42
  Then recibo todos sus campos correctamente poblados
  And updated_at y created_at son timestamps válidos

Scenario: Update observation fields
  Given una observación existente con ID 42 con title "old" y content "old content"
  When actualizo title a "new title" y content a "new content"
  Then el title en DB es "new title"
  And el content en DB es "new content"
  And updated_at se actualiza al momento del cambio
  And revision_count se incrementa en 1
  And el índice FTS5 refleja el nuevo contenido

Scenario: Update type, scope, topic_key
  Given una observación existente con ID 42 con type "general", scope "project"
  When actualizo type a "fix" y scope a "personal" y topic_key a "auth"
  Then type es "fix", scope es "personal", topic_key es "auth"
  And updated_at se actualiza

Scenario: Soft delete observation
  Given una observación existente con ID 42 con deleted_at IS NULL
  When ejecuto DeleteObservation(42, hard=false)
  Then deleted_at se establece con la timestamp actual
  And la observación NO es eliminada físicamente de la tabla
  And la observación NO aparece en RecentObservations

Scenario: Hard delete observation
  Given una observación existente con ID 42
  When ejecuto DeleteObservation(42, hard=true)
  Then la fila es eliminada físicamente de observations
  And el registro correspondiente en observations_fts también se elimina

Scenario: List recent observations with filters
  Given 20 observaciones de distintos proyectos y scopes
  When uso RecentObservations con limit=5, project="Domain"
  Then recibo máximo 5 observaciones
  And todas pertenecen al project "Domain"
  And están ordenadas por created_at DESC

Scenario: Error on get nonexistent ID
  Given no existe observación con ID 9999
  When consulto GetObservation(9999)
  Then recibo un error que contiene "observation not found"

Scenario: Error on delete already-deleted observation (soft)
  Given una observación con ID 42 que ya fue soft-deleted (deleted_at IS NOT NULL)
  When ejecuto DeleteObservation(42, hard=false)
  Then recibo un error que contiene "already deleted"

Scenario: Error on create without required fields
  When creo una observación sin title y sin content
  Then recibo un error que contiene "title and content are required"

Scenario: Conflict detection — AddObservation returns candidates when duplicates exist
  Given existe una observación con title "Bug en login", content "El modal no cierra", project "Domain"
  When creo una observación con title "Bug en login", content "El modal no cierra al hacer submit", project "Domain"
  Then la observación se crea exitosamente
  And el resultado incluye una lista candidates[] con la observación similar existente
  And candidates[0].reason contiene "high_similarity"

Scenario: Prompt capture — save with capture_prompt=true records prompt
  Given hay un prompt activo en el contexto local "fix login modal bug"
  When creo una observación con capture_prompt=true
  Then además de crear la observación, se inserta un registro en user_prompts con ese prompt
  And el campo capture_prompt no se persiste en observations (es un flag de control)
```

## Análisis breve

- **Qué pide realmente:** Implementar las 5 operaciones CRUD del store layer para observations: AddObservation, GetObservation, UpdateObservation, DeleteObservation (soft/hard), RecentObservations. Incluye detección de conflictos por similitud (candidates), captura best-effort de prompt, hashing para dedup, y sync con FTS5 vía triggers (ya existentes de HU-01.1).
- **Módulos sospechados:** `internal/store/` — archivo `observations.go` con las funciones CRUD, más posible archivo `types.go` para estructuras Observation y ObservationFilter
- **Riesgos / dependencias:** Depende de HU-01.1 (schema y migraciones aplicadas); FTS5 triggers ya existen y solo requieren que las operaciones CRUD usen SQL directo; la detección de conflictos depende del algoritmo de similitud (normalized_hash); la captura de prompt requiere un contexto process-local
- **Esfuerzo tentativo:** M

## Verificación previa

- [x] Revisar codebase (grep) — proyecto greenfield, sin Go code aún
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** No existe `go.mod`, no hay archivos `.go` en el repo
- **Acción derivada:** Implementar `internal/store/observations.go` con las 5 operaciones CRUD + tipos + tests
