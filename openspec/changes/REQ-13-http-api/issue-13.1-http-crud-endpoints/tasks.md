# Tasks: issue-13.1-http-crud-endpoints

> Decisión de arquitectura: el proyecto usa handlers EXPLÍCITOS por entidad
> (internal/api/handler/*.go, ~90 rutas en api.go con net/http ServeMux 1.22
> method+pattern routing) en lugar del factory genérico `CRUDHandlers[T]`.
> Razón: cada entidad tiene validaciones y semánticas propias en su service
> (naming explícito > genérico, regla ai-generation.md); la CONSISTENCIA de
> shapes que el factory garantizaría la enforcea el linter issue-13.9
> (response-shape-lint + snapshots de endpoints/error codes en CI).

## Backend

- [x] Entity interface + EntityRegistry + CRUDHandlers[T] factory → N/A por diseño (ver nota; handlers explícitos + linter 13.9)
- [x] Store genérico → N/A; services por feature con pgx (clean-architecture.md)
- [x] Router → net/http ServeMux nativo Go 1.22 (method + path patterns), sin chi/gorilla — menos deps
- [x] Validación de schemas → en service layer por entidad (ErrSlugInvalid, ErrInvalidKind, etc. → 422 con code); go-playground/validator innecesario
- [x] Response envelope `{data}` / `{error}` → writeData/writeError consistentes, enforced por response-shape-lint
- [x] Serialización consistente → JSON tags snake_case (convención api.md, no camelCase), TIMESTAMPTZ ISO8601, UUIDs string
- [x] 404 para inexistentes → anti-enumeration (mismo 404 para no-existe y no-autorizado)
- [x] 422 con código por campo → writeError(422, "validation_failed", detalle)
- [x] PUT vs PATCH → solo PATCH (merge parcial); PUT omitido por convención api.md ("no usar salvo CAS necesario")
- [x] Soft delete con deleted_at → patrón en todas las entidades principales
- [ ] OpenAPI/Swagger spec automática → DIFERIDO a REQ-22 SDK clients (los snapshots testdata/api/endpoints sirven de inventario máquina-leíble mientras tanto)
- [x] GET /health → issue-01.3 (/health, /health/ready, /health/startup)

## Tests

- [x] Factory/registro → N/A por diseño
- [x] Validación por entidad → suites integration por service
- [x] Ciclo CRUD integration → api_integration_test.go + suites por feature (agents, crons, webhooks, policies, etc.)
- [x] Consistencia de envelope entre entidades → response-shape-lint snapshots (TestRealAPI_SnapshotsUpToDate falla CI si una ruta cambia shape)
- [x] Errores 404/422 → cubiertos por handlers tests; 405 lo maneja ServeMux por method pattern
- [x] Sabotaje 200-en-201 → response-shape-lint detecta status codes incorrectos por snapshot

## Cierre

- [x] Verificación → suites integration (mismo código que producción)
- [x] Suite verde → go test ./internal/api/... (2026-06-11)
- [ ] OpenAPI lint → diferido con OpenAPI (REQ-22)
