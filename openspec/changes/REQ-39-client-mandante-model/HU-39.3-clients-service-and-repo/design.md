# Design: HU-39.3-clients-service-and-repo

## Decisión arquitectónica

- **Capa service per entidad**, idéntica al patrón existente en
  `internal/service/project/`. Sin compartir tablas de validación
  globales.
- **Repository interface + pg impl**: permite tests sin Postgres (fake)
  y mantiene el service desacoplado de pgx.
- **Errores domain-level tipados** mapeables a HTTP/MCP. No filtrar
  pgconn.PgError al caller.
- **`WithOrgTx` obligatorio**: el service envuelve cada operación;
  repo recibe `pgx.Tx` (no pool).
- **Cursor pagination simple**: ordenado por `(created_at DESC, id DESC)`,
  con tie-breaker estable.

## Topología

```
┌─ handler REST (HU-39.4) ────────┐
│   POST /api/v1/clients          │
│   → svc.Create(ctx, input)      │
└────────────┬────────────────────┘
             │
┌─ tool MCP (HU-39.5) ───────────┐
│   client.create                 │
│   → svc.Create(ctx, input)      │
└────────────┬────────────────────┘
             │
             ▼
┌─ service.Service ───────────────┐
│   - validate input              │
│   - WithOrgTx(ctx, ...)         │
│   - repo.Insert(tx, row)        │
│   - map pgErr → domain err      │
└────────────┬────────────────────┘
             │
             ▼
┌─ pg_repository (impl) ──────────┐
│   tx.Exec("INSERT INTO clients  │
│            ...")                │
└────────────┬────────────────────┘
             │
             ▼ (RLS filtra por current_org_id())
        Postgres / table clients
```

## Tipos

```go
package client

type Client struct {
    ID             uuid.UUID
    OrganizationID uuid.UUID
    Name           string
    Slug           string
    TaxID          *string
    ContactEmail   *string
    ContactPhone   *string
    Address        *string
    Metadata       map[string]any
    Status         string // active|inactive|archived
    CreatedAt      time.Time
    UpdatedAt      time.Time
    DeletedAt      *time.Time
}

type CreateInput struct {
    Name         string
    Slug         string
    TaxID        *string
    ContactEmail *string
    ContactPhone *string
    Address      *string
    Metadata     map[string]any
}

type UpdateInput struct {
    Name         *string
    TaxID        *string
    ContactEmail *string
    ContactPhone *string
    Address      *string
    Metadata     *map[string]any
    Status       *string
}

type ListFilter struct {
    IncludeArchived bool
    Limit           int    // default 50, max 200
    Cursor          string // opaque, codifica (created_at, id)
}

type ListResult struct {
    Items      []Client
    NextCursor string
}
```

## Interface Repository

```go
type Repository interface {
    Insert(ctx context.Context, tx pgx.Tx, c *Client) error
    GetByID(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Client, error)
    GetBySlug(ctx context.Context, tx pgx.Tx, slug string) (*Client, error)
    List(ctx context.Context, tx pgx.Tx, f ListFilter) (ListResult, error)
    Update(ctx context.Context, tx pgx.Tx, c *Client) error
    Archive(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
    Restore(ctx context.Context, tx pgx.Tx, id uuid.UUID) error
}
```

## Errores

```go
var (
    ErrSlugConflict = errors.New("clients: slug already exists in this organization")
    ErrNotFound     = errors.New("clients: not found")
    ErrInvalidInput = errors.New("clients: invalid input")
)
```

## Validaciones (en service, antes de tocar DB)

| Campo | Regla |
|-------|-------|
| `Name` | trim → len ∈ [1, 255] |
| `Slug` | regex `^[a-z0-9][a-z0-9-]{0,98}[a-z0-9]$` (len 2..100) |
| `ContactEmail` | si presente: regex email simple, len ≤ 255 |
| `Status` (Update) | uno de `active|inactive|archived` |
| `Limit` | clamp a [1, 200], default 50 |

## Mapeo de errores Postgres

```go
func mapPgError(err error) error {
    if errors.Is(err, pgx.ErrNoRows) { return ErrNotFound }
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch pgErr.Code {
        case "23505": // unique_violation
            if pgErr.ConstraintName == "clients_organization_id_slug_key" {
                return ErrSlugConflict
            }
        case "23514": // check_violation
            return ErrInvalidInput
        }
    }
    return err
}
```

## Flujo de Create

```
Service.Create(ctx, in)
  1. orgID = ctx.OrgID()
  2. validate(in) → si falla, return ErrInvalidInput
  3. row = newClient(orgID, in)
  4. WithOrgTx(ctx, pool, orgID, func(tx) {
       return repo.Insert(ctx, tx, row)
     })
  5. mapPgError(err) si error
  6. return row
```

## Decisión sobre `Restore`

- Re-asigna `deleted_at = NULL` y `status = 'active'`.
- Si el slug ya fue tomado por otro cliente nuevo en la misma org,
  el `UPDATE` falla con `23505` → `ErrSlugConflict`. El operador debe
  cambiar slug del registro nuevo o crear uno con sufijo.

## Coverage objetivo

- `service.go`: ≥85% (logica de validación + mapeo errores).
- `pg_repository.go`: cubierto por integration tests (cuenta separada).
- Tests obligatorios:
  - Crear OK / conflict / invalid input.
  - List aislada por org.
  - Archive + List.
  - Restore con/sin conflicto.
  - Cross-tenant lookup → ErrNotFound.
