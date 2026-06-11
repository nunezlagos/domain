# Tasks: issue-03.6-dedup-privacy

## Backend

- [x] `internal/memory/dedup.go`: `Normalize(input string) string` (lowercase, trim, collapse spaces)
- [x] `internal/memory/dedup.go`: `HashObservation(obs Observation) string` (SHA-256 de project_id|scope|type|title|content normalizado)
- [x] `internal/memory/privacy.go`: `StripPrivate(content string) (string, int)` con regex `<private>.*?</private>`
- [x] `migrations/XXXX_add_hash_to_observations.sql`: columna `hash TEXT UNIQUE` + índice
- [x] `migrations/XXXX_create_observation_hashes.sql`: tabla rolling window + FK
- [x] `internal/store/pg/dedup.go`: `DedupStore` interfaz con `Check(hash) (bool, UUID, error)`, `Register(hash, obsID)`, `Cleanup(windowSize int)`
- [x] Integrar dedup check en `MemoryService.SaveObservation`: strip → normalize → hash → check rolling → insert
- [x] Cleanup periódico: cada 50 inserts, DELETE hashes fuera del window
- [x] Logging de bloques privados eliminados (metadata, no contenido)
- [x] Config: rolling_window_size, cleanup_interval

## Tests

- [x] Test unitario: hash normalizado ignora diferencias de mayúsculas y espacios
- [x] Test unitario: StripPrivate elimina tags y devuelve count correcto
- [x] Test unitario: StripPrivate con tags anidados (solo elimina el primero)
- [x] Test unitario: StripPrivate sin tags → content sin cambios
- [x] Test de integración: duplicado → ErrDuplicateObservation + original devuelto
- [x] Test de integración: rolling window cleanup funciona
- [x] Test de integración: unique constraint violada → error manejado
- [x] Sabotaje: dropear unique constraint → app detecta duplicado igual

## Cierre

- [x] Verificación manual: insertar observación duplicada, confirmar error
- [x] Suite verde
