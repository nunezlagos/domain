# REQ-01-core-memory-store: Motor de persistencia principal

**Estado:** activo
**Creado:** 2026-06-07

## Descripción

Motor de persistencia principal basado en SQLite con FTS5 para almacenar, buscar y gestionar observaciones, sesiones, prompts y metadatos del sistema. CRUD completo con soft-delete, hard-delete, deduplicación, upsert por topic_key y saneamiento de privacidad.

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-01.1-database-schema | active | Schema SQLite: 8 tablas, WAL mode, FTS5 triggers, migraciones versionadas |
| issue-01.2-observation-crud | active | CRUD observaciones: AddObservation, GetObservation, UpdateObservation, DeleteObservation, RecentObservations, conflict candidates, prompt capture |
| issue-01.3-fts5-search | active | Búsqueda FTS5: sanitización, filtros type/project/scope, paginación, snippets |
| issue-01.4-deduplication | active | Dedup por hash normalizado SHA-256 en ventana temporal configurable |
| issue-01.5-topic-key-upsert | active | Upsert por topic_key+project+scope con revision_count |
| issue-01.6-prompt-storage | active | CRUD prompts con FTS5, buffer process-local para prompt capture |
| issue-01.7-privacy-stripping | active | Strip `<private>` tags en store layer (defensa en profundidad con plugin) |
| issue-01.8-export-import | active | Export/Import JSON: dump completo/project-scoped, carga atómica con INSERT OR IGNORE |
