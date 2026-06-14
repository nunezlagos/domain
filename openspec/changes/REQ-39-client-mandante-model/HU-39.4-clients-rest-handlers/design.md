# Design: HU-39.4-clients-rest-handlers

## Decisión arquitectónica

- **Handler thin**: decodifica request, llama service, serializa response,
  mapea error → status. No lógica de negocio.
- **Slug-based URLs**: `/clients/{slug}` es la forma canónica. Sin UUIDs
  en path.
- **DTOs separados de entidad**: evita filtrar campos internos por error.
- **Error mapping centralizado** en `respondClientError`.
- **Bootstrap único**: el service se construye una vez en `httpserver.New(...)`
  y se inyecta al handler.

## Alternativas descartadas

- **UUID en path** (`/clients/{id}`): rompe URLs amigables del dashboard.
  Rechazado.
- **Sin DTO** (serializar entidad directamente): filtra detalles internos
  (`deleted_at` en json output) y rompe versionado. Rechazado.
- **Validación dual handler+service**: viola DRY. Rechazado.
- **DELETE = hard delete**: descartado por requisito de soft delete.
  DELETE → Archive.

## Rutas

```
POST    /api/v1/clients                 → Create
GET     /api/v1/clients                 → List
GET     /api/v1/clients/{slug}          → GetBySlug
PATCH   /api/v1/clients/{slug}          → Update
DELETE  /api/v1/clients/{slug}          → Archive
POST    /api/v1/clients/{slug}/restore  → Restore
```

Path params: `slug` (lowercase, len 2..100).

## DTOs

```go
type ClientResponse struct {
    ID             uuid.UUID         `json:"id"`
    OrganizationID uuid.UUID         `json:"organization_id"`
    Name           string            `json:"name"`
    Slug           string            `json:"slug"`
    TaxID          *string           `json:"tax_id,omitempty"`
    ContactEmail   *string           `json:"contact_email,omitempty"`
    ContactPhone   *string           `json:"contact_phone,omitempty"`
    Address        *string           `json:"address,omitempty"`
    Metadata       map[string]any    `json:"metadata"`
    Status         string            `json:"status"`
    CreatedAt      time.Time         `json:"created_at"`
    UpdatedAt      time.Time         `json:"updated_at"`
    DeletedAt      *time.Time        `json:"deleted_at,omitempty"`
}

type CreateClientRequest struct {
    Name         string         `json:"name"`
    Slug         string         `json:"slug"`
    TaxID        *string        `json:"tax_id,omitempty"`
    ContactEmail *string        `json:"contact_email,omitempty"`
    ContactPhone *string        `json:"contact_phone,omitempty"`
    Address      *string        `json:"address,omitempty"`
    Metadata     map[string]any `json:"metadata,omitempty"`
}

type UpdateClientRequest struct {
    Name         *string         `json:"name,omitempty"`
    TaxID        *string         `json:"tax_id,omitempty"`
    ContactEmail *string         `json:"contact_email,omitempty"`
    ContactPhone *string         `json:"contact_phone,omitempty"`
    Address      *string         `json:"address,omitempty"`
    Metadata     *map[string]any `json:"metadata,omitempty"`
    Status       *string         `json:"status,omitempty"`
}

type ListClientsResponse struct {
    Items      []ClientResponse `json:"items"`
    NextCursor string           `json:"next_cursor"`
}

type ErrorBody struct {
    Error   string `json:"error"`
    Message string `json:"message"`
}
```

## Error mapping

```go
func respondClientError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, client.ErrSlugConflict):
        respondJSON(w, 409, ErrorBody{"slug_conflict", err.Error()})
    case errors.Is(err, client.ErrNotFound):
        respondJSON(w, 404, ErrorBody{"not_found", err.Error()})
    case errors.Is(err, client.ErrInvalidInput):
        respondJSON(w, 422, ErrorBody{"invalid_input", err.Error()})
    default:
        log.Error(err)
        respondJSON(w, 500, ErrorBody{"internal_error", "internal error"})
    }
}
```

## Flujo POST /clients

```
1. Auth middleware → ctx con orgID, userID
2. Decode JSON → CreateClientRequest
3. Map → service.CreateInput
4. svc.Create(ctx, input) → Client | err
5. Si err: respondClientError
6. Si OK:
     Location: /api/v1/clients/<slug>
     status 201
     body: toResponse(client)
```

## Wiring (bootstrap)

```go
// internal/httpserver/server.go (extracto conceptual)
clientRepo := client.NewPgRepository(logger)
clientSvc  := client.New(pool, clientRepo, logger)
clientH    := handler.NewClientHandler(clientSvc, logger)

router.Route("/api/v1/clients", func(r chi.Router) {
    r.Use(authMiddleware)
    r.Post("/", clientH.Create)
    r.Get("/", clientH.List)
    r.Get("/{slug}", clientH.GetBySlug)
    r.Patch("/{slug}", clientH.Update)
    r.Delete("/{slug}", clientH.Archive)
    r.Post("/{slug}/restore", clientH.Restore)
})
```

(Sintaxis aproximada; ajustar al router real del proyecto.)

## Listado con paginación

- Query params: `limit` (default 50, max 200), `cursor` (opaco),
  `include_archived` (bool, default false).
- Cursor codifica `(created_at, id)` con base64 — reutilizar
  `internal/api/cursor`.

## Status codes

| Verbo | OK | Error notable |
|-------|-----|---------------|
| POST | 201 Created | 409 slug_conflict, 422 invalid_input |
| GET list | 200 OK | -- |
| GET item | 200 OK | 404 not_found |
| PATCH | 200 OK | 404, 422, 409 (si Update incluyera slug futuro) |
| DELETE | 204 No Content | 404 |
| POST restore | 200 OK | 404, 409 |
| (todas sin Bearer) | -- | 401 unauthorized |
