# Proposal: HU-28.3-middleware-principal-crossorg

## Intención

Eliminar la repetición del parsing de Principal y cross-org guard en handlers. Value objects tipados + helper centralizado.

## Scope

**Incluye:**
- Definir `OrgIDKey` y `UserIDKey` en un package compartido (ej: `internal/api/ctxkeys/`)
- Middleware `principal.Middleware` que post-auth extrae Principal, parsea UUIDs, inyecta en ctx
- Helper `(a *API) authorizeOrg(ctx, resourceOrgID) error` en `handler/api.go`
- Helper `(a *API) orgID(ctx) uuid.UUID` y `(a *API) userID(ctx) uuid.UUID`
- Migración de 5 handlers representativos al nuevo patrón
- Tests unitarios del helper + middleware

**No incluye:**
- Migración de todos los ~30 handlers (se hace progresivamente, HU futura o fuera de REQ-28)
- Eliminación del `principal(r)` legacy

## Estrategia

```go
// internal/api/handler/api.go
func (a *API) orgID(ctx context.Context) uuid.UUID {
    id, _ := ctx.Value(ctxkeys.OrgIDKey).(uuid.UUID)
    return id
}

func (a *API) authorizeOrg(ctx context.Context, resourceOrgID uuid.UUID) error {
    if a.orgID(ctx) == resourceOrgID {
        return nil
    }
    return ErrNotFound // mismo error sentinel que los services
}
```

Migración strangler: handlers existentes siguen usando `principal(r)`. Handlers migrados usan `a.orgID(ctx)` y `a.authorizeOrg(ctx, ...)`. Cuando no queden handlers legacy, se elimina `principal(r)`.
