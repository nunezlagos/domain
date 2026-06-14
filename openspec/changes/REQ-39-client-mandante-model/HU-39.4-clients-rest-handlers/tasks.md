# Tasks: HU-39.4-clients-rest-handlers

## Handler

- [ ] **h-001**: Crear `internal/api/handler/client.go` con struct
      `ClientHandler{svc *client.Service, log *slog.Logger}`.
- [ ] **h-002**: Constructor `NewClientHandler(svc, log)`.
- [ ] **h-003**: Método `Create(w, r)`: decode → svc.Create → respondJSON 201 +
      Location header.
- [ ] **h-004**: Método `List(w, r)`: parse query (limit, cursor,
      include_archived) → svc.List → respondJSON 200.
- [ ] **h-005**: Método `GetBySlug(w, r)`: extrae slug del path → svc.GetBySlug
      → respondJSON 200 o 404.
- [ ] **h-006**: Método `Update(w, r)`: decode PATCH body → svc.Update.
- [ ] **h-007**: Método `Archive(w, r)`: extrae slug → svc.GetBySlug → svc.Archive
      por ID → respondJSON 204.
- [ ] **h-008**: Método `Restore(w, r)`: extrae slug → svc.Restore (admite
      buscar por slug entre archivados) → respondJSON 200.
- [ ] **h-009**: Helper `respondClientError(w, err)` centralizando mapeo.

## DTOs

- [ ] **dto-001**: Definir `ClientResponse`, `CreateClientRequest`,
      `UpdateClientRequest`, `ListClientsResponse`, `ErrorBody` en el
      mismo archivo o en `client_dto.go`.
- [ ] **dto-002**: Helper `toResponse(client.Client) ClientResponse`.

## Wiring

- [ ] **w-001**: En `internal/httpserver/server.go` (o equivalente),
      construir `pgRepo` → `Service` → `Handler` y registrar rutas
      bajo `/api/v1/clients`.
- [ ] **w-002**: Confirmar middleware Bearer aplicado al subrouter.
- [ ] **w-003**: Confirmar que el orden de rutas no causa shadowing
      (`/{slug}/restore` antes que `/{slug}` si el router lo requiere).

## Tests handler (httptest, fake svc)

- [ ] **uh-001**: Fake service `fakeClientService` que implementa la
      misma superficie.
- [ ] **uh-002**: POST con body válido → 201, Location header presente.
- [ ] **uh-003**: POST con body inválido JSON → 400.
- [ ] **uh-004**: POST con slug duplicado (fake retorna ErrSlugConflict)
      → 409.
- [ ] **uh-005**: GET list → 200 con shape correcto.
- [ ] **uh-006**: GET /{slug} not found → 404.
- [ ] **uh-007**: PATCH /{slug} OK → 200 con body actualizado.
- [ ] **uh-008**: DELETE /{slug} → 204 sin body.
- [ ] **uh-009**: POST /{slug}/restore → 200 OK.
- [ ] **uh-010**: Sin Authorization → 401.

## Tests integración HTTP (Postgres real)

- [ ] **int-001**: Bootstrap test server con DB real + auth real.
- [ ] **int-002**: Bearer org_a → POST + GET + PATCH + DELETE round-trip.
- [ ] **int-003**: Bearer org_b NO ve cliente de org_a en GET list.
- [ ] **int-004**: Bearer org_b NO puede GET /{slug-de-org-a} → 404.
- [ ] **int-005**: POST con slug duplicado per-org → 409.
- [ ] **int-006**: DELETE → GET subsiguiente 404; GET ?include_archived=true
      sí.

## Documentación / OpenAPI

- [ ] **doc-001**: Si el repo usa swagger/oapi-codegen, agregar definición
      de paths + schemas en el archivo correspondiente.
- [ ] **doc-002**: Si no hay generación automatica, dejar nota en README
      o `docs/api.md` con ejemplos curl.

## Notas para reviewers

- Cambios en: `internal/api/handler/client.go`, wiring en `httpserver`,
  tests. Sin tocar service ni migrations.
- Validar que el handler NO duplique validaciones del service.
- Confirmar que `respondClientError` cubre todos los errores domain.
- Si el router actual no es chi, ajustar sintaxis pero mantener prefijo
  y verbos.
