# Tasks: HU-39.6-projects-accept-client-slug

## Service.Project — types

- [ ] **t-001**: Agregar `ClientSlug *string` a `CreateInput`.
- [ ] **t-002**: Agregar campo de tipo `presence.Optional[string]` (o
      equivalente) `ClientSlug` a `UpdateInput`.
- [ ] **t-003**: Definir interface `clientResolver` con
      `ResolveSlugInTx(ctx, tx, slug) (uuid.UUID, archived bool, err error)`.
- [ ] **t-004**: Inyectar `clientResolver` en `Service` (constructor
      `New(pool, repo, clientResolver, log)`).
- [ ] **t-005**: Declarar `ErrClientNotFound` y `ErrClientArchived`.

## Service.Client — exponer resolver

- [ ] **r-001**: En `service/client/service.go` agregar método o helper
      `(*Service) ResolveSlugInTx(ctx, tx, slug) (uuid.UUID, bool, error)`
      que busca con `WHERE slug=$1` (sin filtrar deleted_at en el SELECT,
      retornando `archived=true` si soft-deleted).
- [ ] **r-002**: Confirmar que la query usa la misma tx provista (no abre
      otra).

## Service.Project — lógica

- [ ] **l-001**: En `Create`, resolver `ClientSlug` antes de `repo.Insert`.
      Mapear errores resolver → `ErrClientNotFound` / `ErrClientArchived`.
- [ ] **l-002**: En `Update`, según `ClientSlug.IsPresent()` + `IsNull()`:
      - Absent → no tocar.
      - Null → setear `client_id = NULL`.
      - Value → resolver + set.
- [ ] **l-003**: En `repo.Insert`, persistir `ClientID` columna nueva.
- [ ] **l-004**: En `repo.Update`, idem.

## Repository.Project — queries

- [ ] **q-001**: Actualizar `SELECT` de `GetByID`, `GetBySlug`, `List`
      con LEFT JOIN a `clients` para devolver `client_slug` y
      `client_name`.
- [ ] **q-002**: Actualizar `Project` struct con campos `ClientID
      *uuid.UUID`, `ClientSlug *string`, `ClientName *string`.
- [ ] **q-003**: Agregar filtro `ClientID` opcional a `ListFilter` (o
      `ClientSlug` que se resuelve antes).
- [ ] **q-004**: Implementar predicate cursor incluyendo client_id si
      es necesario (probablemente no, ya que el orden sigue por
      created_at/id).

## Handler REST (`internal/api/handler/project.go`)

- [ ] **h-001**: Agregar `client_slug` opcional al `CreateProjectRequest`.
- [ ] **h-002**: Agregar campo con presence semantics a
      `UpdateProjectRequest`.
- [ ] **h-003**: Agregar `client_slug` y `client_name` (nullable) a
      `ProjectResponse`.
- [ ] **h-004**: Soportar query param `?client_slug=` en
      `List(w, r)`. Si presente, resolver vía service o pasar al filter
      según convención.
- [ ] **h-005**: Mapear `ErrClientNotFound` → 422 body
      `{"error":"client_not_found", ...}`.
- [ ] **h-006**: Mapear `ErrClientArchived` → 422 body
      `{"error":"client_archived", ...}`.

## MCP tools (`internal/mcp/server/project_tools.go`)

- [ ] **m-001**: Agregar `client_slug` al input schema de `project.create`,
      `project.update`, `project.list`.
- [ ] **m-002**: Agregar `client_slug` y `client_name` (nullable) al
      output JSON.
- [ ] **m-003**: Mapear los nuevos errores a tool errors tipados.

## Tests unitarios

- [ ] **u-001**: Service.Project Create con ClientSlug=nil → no llama
      resolver.
- [ ] **u-002**: Service.Project Create con ClientSlug válido → resolver
      llamado y client_id seteado.
- [ ] **u-003**: Resolver retorna NotFound → service retorna
      ErrClientNotFound (no oculta).
- [ ] **u-004**: Resolver retorna archived=true → ErrClientArchived.
- [ ] **u-005**: Update con `client_slug: null` (presence + null) →
      client_id = nil.
- [ ] **u-006**: Update con `client_slug` ausente → no toca client_id.

## Tests integración

- [ ] **int-001**: POST /api/v1/projects con client_slug válido → 201
      con response enriquecido.
- [ ] **int-002**: POST con client_slug inexistente → 422
      client_not_found.
- [ ] **int-003**: POST con client_slug archivado → 422 client_archived.
- [ ] **int-004**: POST con client_slug de otra org → 422
      client_not_found (RLS no expone).
- [ ] **int-005**: GET /api/v1/projects/{slug} → response incluye
      client_slug/client_name.
- [ ] **int-006**: GET /api/v1/projects?client_slug=acme-corp filtra
      correctamente.
- [ ] **int-007**: PATCH con `client_slug:null` desasocia.
- [ ] **int-008**: PATCH sin campo client_slug NO toca client_id.
- [ ] **int-009**: MCP project.create con client_slug → idem.

## Wiring

- [ ] **w-001**: En el bootstrap del servidor (httpserver / mcp server),
      inyectar `service.Client` como `clientResolver` al
      `service.Project.New(...)`.
- [ ] **w-002**: Asegurar orden de construcción: client svc antes que
      project svc.

## Notas para reviewers

- Cambios extensivos: project service + repo + handler + mcp tools.
  Es la HU más grande del REQ.
- Verificar que las queries actualizadas no introducen regresiones de
  performance (LEFT JOIN sobre clients es trivial con índices).
- Confirmar que la semántica null-vs-absent en PATCH está cubierta por
  test específico (es la fuente clásica de bugs en APIs JSON).
- Wiring: si bootstrap ya estaba sin DI explícita, esta HU empuja a
  formalizarlo. Aceptable hacer cambio mínimo (constructor con extra
  arg) sin refactor mayor.
