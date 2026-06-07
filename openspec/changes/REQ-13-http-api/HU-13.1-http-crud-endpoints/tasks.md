# Tasks: HU-13.1-http-crud-endpoints

## Backend

- [ ] Definir `Entity` interface genérica con campo `ID() string`
- [ ] Implementar `EntityRegistry` con las 16 entidades
- [ ] Implementar `CRUDHandlers[T]` factory con Create, List, Get, Update, Patch, Delete
- [ ] Implementar Store interface genérica con implementación Postgres
- [ ] Configurar router (chi/gorilla) con subrouters por entidad
- [ ] Implementar validación de schemas por entidad con go-playground/validator
- [ ] Implementar response envelope: `{data, pagination}` y `{error}`
- [ ] Implementar serialización consistente (camelCase, ISO8601, UUID strings)
- [ ] Manejar 404 para entidades/IDs inexistentes
- [ ] Manejar 422 para bodies inválidos con detalles por campo
- [ ] Implementar PUT como reemplazo total vs PATCH como merge parcial
- [ ] Implementar DELETE lógico (soft delete) con campo `deleted_at`
- [ ] Generar OpenAPI/Swagger spec automática
- [ ] Agregar endpoint GET /api/v1/health

## Frontend

- [ ] N/A (API pura, sin UI)

## Tests

- [ ] Test unitario: handler factory registro
- [ ] Test unitario: validación de schemas por entidad
- [ ] Test de integración: Create → Get → Update → Patch → List → Delete cycle
- [ ] Test parametrizado que itera sobre todas las entidades
- [ ] Test de errores: 404, 422, 405 (method not allowed)
- [ ] Test de consistencia de response envelope entre entidades
- [ ] Sabotaje: handler que devuelve 200 en 201 → test cae

## Cierre

- [ ] Verificación manual: curl a cada endpoint contra servidor local
- [ ] Suite verde: `go test ./internal/api/...`
- [ ] OpenAPI spec valida contra linter
