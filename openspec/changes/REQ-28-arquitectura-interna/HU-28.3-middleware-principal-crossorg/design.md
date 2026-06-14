# Design: HU-28.3-middleware-principal-crossorg

## Middleware chain actual → nuevo

```
Antes:
  CORS → versioning → request-log → auth → rate-limit → audit → activity → idempotency → handler
                                                                                        ↑ p = principal(r)

Después:
  CORS → versioning → request-log → auth → rate-limit → principal-ctx → audit → activity → idempotency → handler
                                       (inyecta OrgID/UserID en ctx)                              ↑ a.orgID(ctx)
```

El middleware `principal-ctx` se inserta justo después de auth (donde `Principal` ya está resuelto en el context). No abre tx — eso ya lo hace `apikey.Middleware`.

## Helper signature

```go
// internal/api/handler/api.go

// authorizeOrg verifica que el recurso pertenezca a la org del request.
// Retorna ErrNotFound si no coincide (mismo sentinel que los services).
// Esto evita que handlers filtren recursos de otras orgs.
//
// Uso:
//
//   flow, err := a.FlowService.Get(ctx, id)
//   if err != nil { ... }
//   if err := a.authorizeOrg(ctx, flow.OrganizationID); err != nil {
//       a.writeError(w, http.StatusNotFound, "not_found", "Flow not found")
//       return
//   }
func (a *API) authorizeOrg(ctx context.Context, resourceOrgID uuid.UUID) error {
    if a.orgID(ctx) == resourceOrgID {
        return nil
    }
    return ErrNotFound
}
```
