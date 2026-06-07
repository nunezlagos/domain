# REQ-13-http-api: API REST completa para todas las entidades del plataforma. CRUD, búsqueda, paginación, filtros, auth middleware, formatos JSON.

**Estado:** activo
**Creado:** 2026-06-07

## Descripción

API REST completa para todas las entidades del plataforma. CRUD, búsqueda, paginación, filtros, auth middleware, formatos JSON.

## Criterios de éxito

- CRUD completo para todas las entidades via REST
- Auth middleware con API key validation y RBAC
- Paginación, filtros, ordenamiento en todos los list endpoints
- Idempotency-Key estilo Stripe en POST/PATCH/DELETE con cache 24h y conflict detection
- Bulk batch endpoints (/batch) con modos all_or_nothing/best_effort y respuestas 207 Multi-Status
- Cursor-based pagination opaque con filters_hash y legacy offset deprecated
- HTTP caching: ETag, Last-Modified, Cache-Control per-endpoint, If-None-Match (304), If-Match (412 optimistic)
- API versioning policy: URL versioning, deprecation 12 meses min, RFC 8594 Sunset headers, /api/version endpoint

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-13.1-http-crud-endpoints | proposed | CRUD REST para observations, sessions, projects, skills, agents, flows |
| HU-13.2-http-auth-middleware | proposed | Auth middleware: API key extraction, validation, user context injection |
| HU-13.3-http-pagination-filters | proposed | Paginación cursor+offset, filtros combinables, ordenamiento |
| HU-13.4-idempotency-keys | proposed | Idempotency-Key middleware con cache 24h, body hash, SELECT FOR UPDATE concurrency |
| HU-13.5-bulk-batch-endpoints | proposed | POST/DELETE /batch con Multi-Status, all_or_nothing/best_effort modes, max 5000 items |
| HU-13.6-cursor-pagination | proposed | Cursor opaque base64url con filters_hash, sort stable, legacy offset deprecated |
| HU-13.7-http-caching-etags | proposed | ETag SHA-256 + Cache-Control per-endpoint, If-None-Match 304, If-Match 412 optimistic |
| HU-13.8-api-versioning-policy | proposed | URL versioning, sunset RFC 8594, /api/version endpoint, changelog enforcement CI |
