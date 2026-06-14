# Design: issue-25.14-rls-http-wiring

## Arquitectura

```
HTTP request
    │
    ▼
┌─────────────────────────────────────────────┐
│ auth/apikey.Middleware                      │
│   1. extrae Bearer token                    │
│   2. Resolver.Resolve() → Principal         │
│   3. NUEVO: txctx.WithTxContext(ctx, tx)    │
│      donde tx = pool.BeginTx + SET LOCAL    │
│   4. ctx = context.WithValue(ctx, ..., tx)   │
│   5. defer tx.Rollback (no-op si Commit)     │
│   6. next.ServeHTTP(w, r)                    │
│   7. Si handler no hace Commit explícito,   │
│      defer Rollback al salir del wrapper    │
└─────────────────────────────────────────────┘
    │
    ▼
Handler (ej: observation.go)
    │ tx := txctx.TxFromContext(ctx)
    │   ↓ si nil → fallback a pool directo (legacy)
    │   ↓ si tx → usar tx para todas las queries
    ▼
Service (ej: observation.Service.List)
    │ s.pool.Query → ahora usa tx.Query si está en ctx
    ▼
Postgres
    │ tx ya tiene SET LOCAL → RLS deja pasar rows de la org
    ▼
Response
```

## Componentes nuevos

### `internal/store/txctx/context.go`

```go
package txctx

import (
    "context"
    "github.com/jackc/pgx/v5"
)

type txKey struct{}

// WithTxContext injecta la tx en el ctx para que repos la extraigan.
func WithTxContext(ctx context.Context, tx pgx.Tx) context.Context {
    return context.WithValue(ctx, txKey{}, tx)
}

// TxFromContext retorna la tx si fue inyectada por el middleware, sino nil.
// Repos: si nil, usan s.pool (legacy path) o txctx.WithOrgTx (auto-wrap).
func TxFromContext(ctx context.Context) pgx.Tx {
    tx, _ := ctx.Value(txKey{}).(pgx.Tx)
    return tx
}

// MustTxFromContext falla si no hay tx (modo estricto para endpoints RLS).
func MustTxFromContext(ctx context.Context) (pgx.Tx, bool) {
    tx := TxFromContext(ctx)
    return tx, tx != nil
}
```

### `internal/auth/apikey/middleware.go` (modificación)

```go
func (m *Middleware) Wrap(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // ... existing allowlist + bearer extraction ...
        
        p, err := m.Resolver.Resolve(r.Context(), token)
        if err != nil { writeUnauthorized(...); return }
        
        orgID, _ := uuid.Parse(p.OrganizationID)
        userID, _ := uuid.Parse(p.UserID)
        
        // NUEVO: abrir tx con SET LOCAL
        var tx pgx.Tx
        if m.Pool != nil && orgID != uuid.Nil {
            tx, err = m.Pool.BeginTx(r.Context(), pgx.TxOptions{})
            if err != nil { writeInternalError(w); return }
            defer func() { _ = tx.Rollback(r.Context()) }()
            
            if _, err := tx.Exec(r.Context(),
                `SELECT set_config('app.current_org_id', $1, true), set_config('app.current_user_id', $2, true)`,
                orgID.String(), userID.String()); err != nil {
                writeInternalError(w); return
            }
        }
        
        ctx := context.WithValue(r.Context(), principalKey{}, p)
        if tx != nil {
            ctx = txctx.WithTxContext(ctx, tx)
        }
        next.ServeHTTP(w, r.WithContext(ctx))
        // tx auto-rollback si handler no hizo Commit
    })
}
```

## Patrón en repos

```go
// internal/service/observation/service.go
func (s *Service) List(ctx context.Context, projectSlug string) ([]Observation, error) {
    if tx := txctx.TxFromContext(ctx); tx != nil {
        return s.listWithTx(ctx, tx, projectSlug)
    }
    return s.listWithPool(ctx, projectSlug) // legacy fallback
}

func (s *Service) listWithTx(ctx context.Context, tx pgx.Tx, projectSlug string) ([]Observation, error) {
    // ... usa tx.Query en vez de s.pool.Query
}
```

## MCP server wireup

El MCP server no tiene HTTP request. En su lugar, cuando un tool MCP es invocado:
- Ya hay un Principal en ctx (puesto por `mcp/server/auth.go` al validar el dominio).
- Se aplica el mismo patrón: si la org/user están en ctx, abrir tx + SET LOCAL + inyectar en ctx.
- Implementación: helper `mcp/server/wireup.go` con `WithOrgTxForPrincipal(ctx, pool, principal, fn)`.

## Tradeoffs

| Opción | Pros | Contras |
|---|---|---|
| Wireup en middleware HTTP (esta propuesta) | Transparente para handlers; tests E2E cubren el camino real | Refactor de 11 archivos |
| Wireup explícito en cada handler (helpers `withOrgTx`) | Cambio chico por archivo | Fácil olvidar; defense-in-depth roto si se olvida uno |
| BYPASSRLS para app_user + RLS sin FORCE | Cero refactor de handlers | Anula el defense-in-depth; NO recomendado |

## TDD detallado

1. `txctx/context_test.go` unit: `WithTxContext` round-trip, `TxFromContext` nil/ok.
2. `internal/store/txctx/txctx_http_e2e_test.go` integration:
   - testcontainers + HTTP server completo (httptest.NewServer con el router real)
   - 2 orgs, 2 API keys, 2 observations
   - Assert GET con key A → 1 obs (la de A), id de B → 404
3. `internal/mcp/server/memory_tools_e2e_test.go`: mismo patrón pero via MCP tool call.
4. Sabotaje en `txctx_http_e2e_test.go`: handler monkey-patch que ignora tx del ctx y usa pool directo → RLS sigue bloqueando (0 rows).
