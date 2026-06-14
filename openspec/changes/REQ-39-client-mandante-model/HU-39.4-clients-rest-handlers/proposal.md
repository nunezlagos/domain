# Proposal: HU-39.4-clients-rest-handlers

## Intención

Exponer el `service.Client` (HU-39.3) vía endpoints REST estándar bajo
`/api/v1/clients`, con auth Bearer, scoping multi-tenant por org del
caller, validación delegada al service, y mapeo consistente de errores
domain → HTTP.

## Scope

**Incluye:**
- `internal/api/handler/client.go` — handler `ClientHandler` con:
  - `POST /api/v1/clients` → Create
  - `GET /api/v1/clients` → List (con `limit`, `cursor`, `include_archived`)
  - `GET /api/v1/clients/{slug}` → GetBySlug
  - `PATCH /api/v1/clients/{slug}` → Update
  - `DELETE /api/v1/clients/{slug}` → Archive
  - `POST /api/v1/clients/{slug}/restore` → Restore
- Registro del handler en el router de `httpserver` (donde están los demás).
- Wiring del `service.Client.New(...)` en bootstrap.
- DTOs request/response separados de la entidad de dominio.
- Mapeo de errores: `ErrSlugConflict` → 409, `ErrNotFound` → 404,
  `ErrInvalidInput` → 422, otros → 500.
- Tests de handler (httptest) con servicio fake.
- Tests de integración HTTP contra servidor full (Postgres real).
- Documentación OpenAPI mínima (si el repo ya genera spec; si no, queda
  como tarea futura).

**No incluye:**
- Tools MCP equivalentes → HU-39.5.
- Cambios en endpoints de `projects` → HU-39.6.
- Frontend / dashboard UI.
- Cambios en SDK clients (Python/TS) → REQ-22 separado.

## Enfoque técnico

1. **Convención de handler**: replicar el patrón de
   `internal/api/handler/project.go`. Struct con dependencias inyectadas
   (service, logger), métodos por verbo + path.
2. **Slug en path**: idempotente, deep-linkable. UUID disponible como
   campo de respuesta solamente.
3. **DTOs**:
   ```
   ClientResponse {
     id, organization_id, name, slug, tax_id, contact_email,
     contact_phone, address, metadata, status, created_at,
     updated_at, deleted_at
   }
   CreateClientRequest { name, slug, tax_id?, contact_email?, ... }
   UpdateClientRequest { name?, tax_id?, contact_email?, status?, ... }
   ListClientsResponse { items: [...], next_cursor: "" }
   ```
4. **Decode body**: usar el helper estándar del repo (`json.NewDecoder`
   con `DisallowUnknownFields`).
5. **Error mapping** centralizado en helper `respondClientError(w, err)`.
6. **Tests**:
   - Handler con fake service: cubre serialización, status codes,
     decoding.
   - Integración HTTP: arranca servidor con DB real, prueba flujos
     end-to-end con Bearer real.

## Riesgos

- **Doble validación**: si el handler valida slug además del service, se
  duplica regla. Mitigación: handler NO valida, delega al service.
- **Route conflict** con rutas existentes: hay que confirmar que
  `/api/v1/clients` no choca con nada actual. Mitigación: grep al router
  antes de implementar.
- **Bypass de org context**: si el handler usa el pool directo sin pasar
  por el service, RLS no aplica. Mitigación: handler SOLO llama al service.
- **`{slug}` ambiguo con `restore` subpath**: gorilla/chi routing distingue
  `/clients/{slug}` de `/clients/{slug}/restore`. Mitigación: registrar
  `restore` antes de `{slug}` o usar pattern matching explícito.

## Testing

- Test handler aislado (fake service):
  - POST OK / conflict / validation error.
  - GET por slug OK / not found.
  - PATCH OK / not found / validation error.
  - DELETE OK / not found.
  - List con paginación cursor.
  - 401 sin auth.
- Test integración HTTP:
  - Servidor real arriba con DB.
  - Bearer token real para org_a.
  - POST + GET + PATCH + DELETE round-trip.
  - GET desde Bearer de org_b NO ve clients de org_a.
  - POST con slug duplicado → 409.
- Coverage handler ≥75%.
