# Tasks: HU-39.5-clients-mcp-tools

## Tools

- [ ] **tool-001**: Crear `internal/mcp/server/client_tools.go`.
- [ ] **tool-002**: Implementar `clientCreateTool(svc *client.Service)
      mcp.Tool` con InputSchema según design.md.
- [ ] **tool-003**: Implementar `clientListTool(svc)`.
- [ ] **tool-004**: Implementar `clientGetTool(svc)`.
- [ ] **tool-005**: Implementar `clientUpdateTool(svc)`.
- [ ] **tool-006**: Implementar `clientArchiveTool(svc)`.
- [ ] **tool-007**: Implementar `clientRestoreTool(svc)`.
- [ ] **tool-008**: Helper `decodeClientArgs[T any](args map[string]any,
      out *T) error` (o reusar el helper existente del package).
- [ ] **tool-009**: Helper `toClientToolError(err error)` que mapea
      errores domain → tool error.
- [ ] **tool-010**: Helper `clientResultJSON(c *client.Client)` que
      serializa la entidad al payload del result.

## Registro en server

- [ ] **reg-001**: En `internal/mcp/server/server.go` (o donde se haga
      el bootstrap de tools), agregar la función `registerClientTools(reg,
      svc)`.
- [ ] **reg-002**: Llamarla desde `registerAllTools(reg, services)`.
- [ ] **reg-003**: Asegurar que `Services` struct (DI container) incluye
      `Client *client.Service`. Si no existe, agregarlo en el bootstrap
      del MCP server.

## Tests integración

- [ ] **int-001**: Levantar MCP server in-process con DB real (test
      helper).
- [ ] **int-002**: `tools/list` retorna 6 tools nuevos.
- [ ] **int-003**: `tools/call client.create` desde Bearer org_a →
      row creada bajo org_a.
- [ ] **int-004**: `tools/call client.list` desde org_a NO ve clients
      de org_b.
- [ ] **int-005**: `tools/call client.get` con slug de otra org →
      tool error `not_found`.
- [ ] **int-006**: `tools/call client.update` parcial cambia solo el
      campo dado.
- [ ] **int-007**: `tools/call client.archive` + verificación de que
      `client.list` default ya no lo muestra.
- [ ] **int-008**: `tools/call client.restore` revive el cliente.
- [ ] **int-009**: Args inválidos (sin `name` o `slug` malformado) →
      tool error `invalid_input`.

## Tests unitarios

- [ ] **u-001**: Decode args helper round-trip.
- [ ] **u-002**: `toClientToolError` cubre los 4 caminos (slug_conflict,
      not_found, invalid_input, internal_error).

## Notas para reviewers

- Cambios SOLO en `internal/mcp/server/client_tools.go` + wiring mínimo
  en server.go. Sin tocar service, sin tocar handler REST.
- Confirmar que el patrón sigue `project_tools.go` y `issue_tools.go`
  para coherencia con el resto del catálogo.
- Si el SDK MCP del repo cambió entre REQ-31 y hoy, ajustar imports y
  tipos (`mcp.Tool`, `mcp.ToolError`, etc.).
