# Proposal: HU-39.6-projects-accept-client-slug

## Intención

Extender `service.Project`, los handlers REST y los tools MCP de
projects para aceptar `client_slug` como input opcional, resolver
slug→UUID dentro de la org del caller, y exponer `client_slug` /
`client_name` en outputs. Cierra el ciclo del REQ permitiendo
"proyectos por cliente" end-to-end.

## Scope

**Incluye:**
- `internal/service/project/service.go` (+ `repository.go`,
  `pg_repository.go`): agregar `ClientSlug` a `CreateInput` y
  `UpdateInput`, resolución slug→UUID, persistencia de `client_id`.
- Nuevo método interno o inyección de dependencia: `service.Project`
  depende de `service.Client` para resolver slugs (interface mínima).
- Update de queries SELECT en project repo para hacer JOIN/LEFT JOIN
  con `clients` y devolver `client_slug`/`client_name`.
- `internal/api/handler/project.go`: agregar `client_slug` en DTOs
  request/response; agregar query param filter `?client_slug=`.
- `internal/mcp/server/project_tools.go`: idem para tools
  `project.create`, `project.update`, `project.list`, `project.get`.
- Mapeo de errores: `ErrClientNotFound`, `ErrClientArchived` →
  422 / tool error.
- Tests:
  - Unit: validación de payload con client_slug.
  - Integración: round-trip de creación con cliente, sin cliente,
    cross-org (rechazado), archivado (rechazado).

**No incluye:**
- Modificación del schema (ya hecho en 39.2).
- Modificación del service.Client (ya hecho en 39.3).
- Dashboard UI.

## Enfoque técnico

1. **Dependencia mínima `service.Project → service.Client`**: usar una
   interface local `clientResolver` con un solo método
   `ResolveSlug(ctx, slug) (uuid.UUID, archived bool, error)`. La
   implementación delega a `service.Client.GetBySlug` (incluyendo
   soft-deleted). Evita acoplamiento fuerte al package completo.
2. **Resolución dentro de la misma tx**: el `Create` y `Update` de
   project siguen usando `WithOrgTx`. La resolución del slug se hace
   con un repo helper que ejecuta dentro de la misma tx (no se llama al
   service.Client.GetBySlug porque abre otra tx). Patrón: pasar `pgx.Tx`
   al resolver.
3. **Query SELECT con LEFT JOIN**:
   ```sql
   SELECT p.id, p.organization_id, p.client_id, p.name, p.slug, ...,
          c.slug AS client_slug, c.name AS client_name
   FROM projects p
   LEFT JOIN clients c ON c.id = p.client_id AND c.deleted_at IS NULL
   WHERE p.deleted_at IS NULL AND ...
   ```
4. **DTO update**:
   ```
   ProjectResponse: + client_slug, client_name (ambos nullable)
   CreateProjectRequest: + client_slug? (string, optional)
   UpdateProjectRequest: + client_slug? (json.RawMessage or *string para
     distinguir ausente vs null explícito)
   ```
5. **Distinguir "ausente" vs "null"**: usar `*string` + custom unmarshal
   o `json.RawMessage` para detectar `null` explícito (desasocia) vs no
   enviado (no toca client_id).
6. **Filtro en List**: `?client_slug=<slug>` → resuelve a UUID y filtra.
   Si el cliente no existe → 422.
7. **MCP**: input schema añade `client_slug` opcional. Tools `project.list`
   acepta `client_slug` también.

## Riesgos

- **TOCTOU**: si el cliente se borra entre resolución y INSERT, el
  trigger DB falla (FK con SET NULL). Mitigación: misma tx + RLS;
  además el trigger same-org sigue como red.
- **Distinguir null vs absent en JSON**: si no se maneja con cuidado,
  PATCH puede borrar accidentalmente client_id. Mitigación: tipo
  `*string` con doble pointer `**string` no es elegante; usar campo
  bool `client_slug_set` o `json.RawMessage` y parsear manualmente.
  Mantener test específico para ambos casos.
- **JOIN extra penaliza performance**: agregar LEFT JOIN en cada
  listado de projects. Mitigación: índice ya existe (`projects_client_id_idx`),
  y la escala es 20 users. Trivial.
- **MCP schema rompe clientes viejos**: agregar campo opcional no rompe
  contratos existentes (additionalProperties: false ya rechaza extras,
  pero el cliente viejo solo no enviaría client_slug). OK.

## Testing

- Unit: validar shape de `CreateInput` con/sin `ClientSlug`.
- Unit: `clientResolver` fake retorna ErrNotFound → service mapea a
  `ErrClientNotFound` con código tipado.
- Integración:
  - Create sin client_slug → client_id NULL.
  - Create con client_slug válido → client_id correcto + response
    enriquecido.
  - Create con client_slug de otra org → 422 (ErrClientNotFound).
  - Create con client_slug de cliente archivado → 422
    (ErrClientArchived).
  - PATCH con `"client_slug": null` → desasocia.
  - PATCH sin `client_slug` field → no toca client_id.
  - List filter por client_slug → solo proyectos de ese cliente.
- Test MCP equivalente.
