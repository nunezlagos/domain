# Tasks: issue-03.6-dedup-privacy

## Backend

- [ ] `internal/memory/dedup.go`: `Normalize(input string) string` (lowercase, trim, collapse spaces)
- [ ] `internal/memory/dedup.go`: `HashObservation(obs Observation) string` (SHA-256 de project_id|scope|type|title|content normalizado)
- [ ] `internal/memory/privacy.go`: `StripPrivate(content string) (string, int)` con regex `<private>.*?</private>`
- [ ] `migrations/XXXX_add_hash_to_observations.sql`: columna `hash TEXT UNIQUE` + índice
- [ ] `migrations/XXXX_create_observation_hashes.sql`: tabla rolling window + FK
- [ ] `internal/store/pg/dedup.go`: `DedupStore` interfaz con `Check(hash) (bool, UUID, error)`, `Register(hash, obsID)`, `Cleanup(windowSize int)`
- [ ] Integrar dedup check en `MemoryService.SaveObservation`: strip → normalize → hash → check rolling → insert
- [ ] Cleanup periódico: cada 50 inserts, DELETE hashes fuera del window
- [ ] Logging de bloques privados eliminados (metadata, no contenido)
- [ ] Config: rolling_window_size, cleanup_interval

## Tests

- [ ] Test unitario: hash normalizado ignora diferencias de mayúsculas y espacios
- [ ] Test unitario: StripPrivate elimina tags y devuelve count correcto
- [ ] Test unitario: StripPrivate con tags anidados (solo elimina el primero)
- [ ] Test unitario: StripPrivate sin tags → content sin cambios
- [ ] Test de integración: duplicado → ErrDuplicateObservation + original devuelto
- [ ] Test de integración: rolling window cleanup funciona
- [ ] Test de integración: unique constraint violada → error manejado
- [ ] Sabotaje: dropear unique constraint → app detecta duplicado igual

## Cierre

- [ ] Verificación manual: insertar observación duplicada, confirmar error
- [ ] Suite verde
