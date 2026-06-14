# Tasks: issue-02.6-activity-log

## Backend

- [x] Tabla activity_log particionable con visibility + metadata JSONB → migration 000027
- [x] PGStore Record/List con Filter (org/project/actor/entity/cursor) → internal/activity/activity.go
- [x] NopRecorder para tests
- [x] Endpoint GET /api/v1/activity-logs → handler/activity.go
- [x] Middleware HTTP auto-record en mutaciones exitosas (post-auth, excluye /auth/*, preserva Flusher para SSE) → activity.HTTPMiddleware en stack de cmd/domain — 2026-06-10
- [x] Helper de summaries human-readable consistentes → activity.Summarize (action/entity/id/summary desde method+path) — 2026-06-10
- [x] Retention: sin purge por default (decisión de producto; soft-delete general va por issue-23.2)

## Frontend

- [x] N/A (API-first; feed UI en REQ-16)

## Tests

- [x] Test unitario → middleware_test.go (9 tests: matrix Summarize, records mutations, skips reads/errores/auth/no-principal, Flusher preservado, singularize)
- [x] Test E2E → activity_integration_test.go (7 tests testcontainers: record happy path, filtros, ordering, metadata)
- [x] Sabotaje → TestSabotage_Metadata_AcceptsAnyJSON + TestMiddleware_SkipsReadsErrorsAndAuth (mutación fallida NO genera activity)

## Cierre

- [x] Verificación manual → cubierto por suite integración + middleware unit end-to-end
- [x] Suite verde → 2026-06-10
