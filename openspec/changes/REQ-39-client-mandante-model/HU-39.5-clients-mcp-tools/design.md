# Design: HU-39.5-clients-mcp-tools

## Decisión arquitectónica

- **Una tool por operación CRUD**: `create / list / get / update /
  archive / restore`. Sin "supertool" con `op` dinámico.
- **Reutilizar service compartido**: el mismo `*client.Service` del
  handler REST. Sin segundo paths de dominio.
- **JSON Schema inline**: declarado en el mismo archivo que registra el
  tool. Sin generación externa.
- **Naming `client.<verb>`**: dot-namespacing alineado con
  `project.<verb>`, `issue.<verb>` ya existentes.
- **Output JSON-stringified en `text`**: alinea con `project_tools.go`.

## Alternativas descartadas

- **Single tool `client` con discriminador `op`**: peor descubrimiento,
  schemas mezclados, más difícil de validar. Rechazado.
- **Tools llamando SQL directo**: rompe DRY, salta validación,
  rompe RLS si olvida WithOrgTx. Rechazado.
- **Output como `resource` con uri**: posible a futuro para deep linking
  desde dashboard, queda fuera de esta HU.

## Catálogo de tools

| Tool | Input (resumen) | Output |
|------|-----------------|--------|
| `client.create` | name (req), slug (req), tax_id?, contact_email?, contact_phone?, address?, metadata? | Client JSON |
| `client.list` | limit? (int, default 50, max 200), cursor?, include_archived? (bool) | `{ items: [Client], next_cursor: string }` |
| `client.get` | slug (req) | Client JSON o error not_found |
| `client.update` | slug (req), name?, tax_id?, contact_email?, contact_phone?, address?, metadata?, status? | Client JSON actualizado |
| `client.archive` | slug (req) | `{ ok: true, slug }` |
| `client.restore` | slug (req) | Client JSON con status='active' |

## Input schemas (ejemplo: client.create)

```json
{
  "type": "object",
  "required": ["name", "slug"],
  "additionalProperties": false,
  "properties": {
    "name":          { "type": "string", "minLength": 1, "maxLength": 255 },
    "slug":          { "type": "string", "pattern": "^[a-z0-9][a-z0-9-]{0,98}[a-z0-9]$" },
    "tax_id":        { "type": ["string", "null"], "maxLength": 50 },
    "contact_email": { "type": ["string", "null"], "format": "email" },
    "contact_phone": { "type": ["string", "null"], "maxLength": 50 },
    "address":       { "type": ["string", "null"] },
    "metadata":      { "type": "object" }
  }
}
```

(Schemas equivalentes para los demás tools.)

## Error mapping

```go
func toToolError(err error) mcp.ToolError {
    switch {
    case errors.Is(err, client.ErrSlugConflict):
        return mcp.ToolError{Code: "slug_conflict", Message: err.Error()}
    case errors.Is(err, client.ErrNotFound):
        return mcp.ToolError{Code: "not_found", Message: err.Error()}
    case errors.Is(err, client.ErrInvalidInput):
        return mcp.ToolError{Code: "invalid_input", Message: err.Error()}
    default:
        return mcp.ToolError{Code: "internal_error", Message: "internal error"}
    }
}
```

(Si el SDK MCP del repo no tiene `ToolError`, ajustar al wrapper local
ya usado en `project_tools.go`.)

## Flujo de un tool (ejemplo: client.create)

```
1. mcp middleware → ctx con orgID extraído del Bearer
2. tool.Handler(ctx, args):
   a. decodeArgs(args, &CreateInput) → si JSON Schema falla antes, no llega aquí
   b. svc.Create(ctx, in) → Client | err
   c. if err: toToolError(err)
   d. return result{ content: [{ type:"text", text: json(Client) }] }
```

## Wiring

```go
// internal/mcp/server/server.go (extracto conceptual)
func registerAllTools(reg *toolRegistry, svc Services) {
    registerProjectTools(reg, svc.Project)
    registerIssueTools(reg, svc.Issue)
    registerClientTools(reg, svc.Client)   // ← nuevo
    ...
}

func registerClientTools(reg *toolRegistry, svc *client.Service) {
    reg.Add(clientCreateTool(svc))
    reg.Add(clientListTool(svc))
    reg.Add(clientGetTool(svc))
    reg.Add(clientUpdateTool(svc))
    reg.Add(clientArchiveTool(svc))
    reg.Add(clientRestoreTool(svc))
}
```

## Decisión sobre `restore` y archive

- Ambas reciben `slug` (no UUID) para consistencia con REST.
- El service hace `GetBySlug` (incluyendo soft-deleted en el caso de
  restore) → opera sobre el ID.

## Coverage objetivo

- `client_tools.go`: ≥75% (cubre 6 tools + helpers).
- Test obligatorio: cada tool con happy path + 1 error path tipado.
