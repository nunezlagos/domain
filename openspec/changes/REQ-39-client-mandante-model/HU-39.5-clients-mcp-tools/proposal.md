# Proposal: HU-39.5-clients-mcp-tools

## Intención

Exponer las operaciones de `service.Client` como MCP tools vía
Streamable HTTP (REQ-31), siguiendo el patrón de
`internal/mcp/server/project_tools.go`. Cada tool tiene input/output
schema JSON Schema, error mapping consistente, y comparte el mismo
service que el handler REST.

## Scope

**Incluye:**
- `internal/mcp/server/client_tools.go` con tools:
  - `client.create`
  - `client.list`
  - `client.get`
  - `client.update`
  - `client.archive`
  - `client.restore`
- Registro de tools en el catálogo MCP existente (función `register*` que
  ya emplea el server).
- Wiring del `service.Client` en el bootstrap del MCP server (si no
  llegaba desde HU-39.4; típicamente comparten un container DI).
- JSON Schemas inline para cada tool input/output.
- Mapeo error domain → tool error MCP estándar.
- Tests de integración tool-by-tool con DB real.

**No incluye:**
- Cambios al protocolo MCP (REQ-31 ya cubrió Streamable HTTP).
- Documentación de usuarios MCP externos.
- Tools para `projects` con `client_id` → HU-39.6.

## Enfoque técnico

1. **Patrón de tool**: cada tool es una función
   `clientCreateTool(svc *client.Service) mcp.Tool` que retorna:
   ```
   mcp.Tool{
     Name:        "client.create",
     Description: "...",
     InputSchema: <JSON schema>,
     Handler:     func(ctx, args) (result, error) { ... }
   }
   ```
2. **Decode args**: cada handler decodifica `args` (map[string]any) a un
   struct tipado vía `json.Marshal` round-trip o helper `decodeArgs`.
3. **Llamada al service**: idéntico al handler REST. Reutilizar las
   mismas operaciones (Create, List, etc.).
4. **Mapeo de errores**:
   - `ErrSlugConflict` → tool error `slug_conflict`.
   - `ErrNotFound` → tool error `not_found`.
   - `ErrInvalidInput` → tool error `invalid_input`.
   - resto → tool error `internal_error`.
5. **Output content**: estructura JSON dentro de `content[]` con type
   `text` (JSON stringified) o `resource` según convención del repo.
   Imitar `project_tools.go`.
6. **Registro**: añadir el constructor de tools a la función de bootstrap
   del MCP server (donde se llaman `registerProjectTools`, etc.).

## Riesgos

- **Schema drift entre REST y MCP**: si los DTOs HTTP cambian, los
  schemas MCP pueden quedar desfasados. Mitigación: deliberadamente
  mismos nombres de campo (snake_case) y revisión cruzada en code review.
- **Olvido de registro en catalog**: tool definido pero no listado.
  Mitigación: test que verifica `tools/list` incluye los 6 nuevos.
- **Bearer MCP no propaga org**: si el middleware MCP no setea orgID en
  ctx, las queries fallan por RLS. Mitigación: REQ-31 ya lo cubre;
  validar en test que orgID llega.
- **Duplicación de validación**: los schemas JSON Schema validan
  estructura, pero la validación semántica (slug regex) sigue siendo del
  service. Mitigación: schemas declaran tipos/required; el service hace
  el resto.

## Testing

- Test integración tools (Postgres + MCP server in-process):
  - `tools/list` incluye los 6.
  - `client.create` → row creada en DB.
  - `client.list` desde org_a NO ve org_b.
  - `client.get` not-found cross-tenant.
  - `client.archive` + `client.restore` round-trip.
  - args inválidos → tool error invalid_input.
- Coverage ≥75% para client_tools.go.
- Test e2e con `curl` Streamable HTTP opcional (siguiendo el patrón de
  REQ-31).
