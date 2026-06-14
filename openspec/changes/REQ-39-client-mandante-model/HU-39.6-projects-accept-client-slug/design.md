# Design: HU-39.6-projects-accept-client-slug

## Decisión arquitectónica

- **Resolución slug→UUID en el service**, no en el handler. Mantiene
  contrato simple: callers solo conocen slugs.
- **Misma tx para resolución + INSERT/UPDATE**: evita TOCTOU. El resolver
  recibe `pgx.Tx`.
- **LEFT JOIN para devolver client info**: el response incluye
  `client_slug` y `client_name` cuando hay cliente; null si no.
- **Distinción explícita absent vs null en PATCH**: campo
  `*string` no alcanza; se usa `json.RawMessage` o helper de "presence"
  para diferenciar "no enviado" (no tocar) de "enviado como null"
  (desasociar).
- **Errores nuevos tipados** en el service.Project:
  `ErrClientNotFound`, `ErrClientArchived`.

## Alternativas descartadas

- **Resolver en el handler**: rompe DRY y obliga a tener
  `service.Client` accesible desde cada handler que cree projects.
  Rechazado.
- **Aceptar `client_id` UUID en input**: rompe UX (URLs y curl con
  UUIDs). Slug es la convención. UUID sigue siendo válido vía service
  layer interno.
- **Resolución vía CTE en el mismo INSERT** (`WITH c AS (SELECT id FROM
  clients WHERE slug=$1) INSERT INTO projects (..., client_id) SELECT
  ..., c.id FROM c`): elegante pero pierde la posibilidad de mapear
  "cliente archivado" vs "cliente inexistente" como errores distintos
  desde la DB. Rechazado por DX peor.

## Cambios al service.Project

```go
// service.go
type CreateInput struct {
    Name       string
    Slug       string
    // ... campos previos
    ClientSlug *string // si nil → client_id NULL
}

type UpdateInput struct {
    Name       *string
    // ... campos previos
    ClientSlug presence.Optional[string] // nil-vs-absent
}

type clientResolver interface {
    ResolveSlugInTx(ctx context.Context, tx pgx.Tx, slug string) (id uuid.UUID, archived bool, err error)
}

type Service struct {
    pool      *pgxpool.Pool
    repo      Repository
    clients   clientResolver
    log       *slog.Logger
}
```

`presence.Optional[T]` es un helper que distingue:
- Absent (zero value)
- Present + nil (interpretar como JSON null → desasociar)
- Present + value (asignar)

(O implementación equivalente con `json.RawMessage` + flag.)

## Flujo Create

```
1. validate(in) — incluye nombre/slug
2. WithOrgTx(ctx, pool, orgID, func(tx):
     a. var clientID *uuid.UUID
     b. if in.ClientSlug != nil:
          id, archived, err := clients.ResolveSlugInTx(ctx, tx, *in.ClientSlug)
          if err == ErrClientNotFound: return ErrClientNotFound
          if archived: return ErrClientArchived
          clientID = &id
     c. row.ClientID = clientID
     d. repo.Insert(ctx, tx, row)
   )
3. mapPgError
4. enrich response (client_slug si clientID != nil)
```

## Flujo Update

```
1. WithOrgTx ...
2. existing := repo.GetByID(...)
3. apply patch (incluyendo:)
     if in.ClientSlug.IsPresent():
        if in.ClientSlug.IsNull():
           existing.ClientID = nil
        else:
           id, archived, err := clients.ResolveSlugInTx(...)
           if archived: return ErrClientArchived
           existing.ClientID = &id
4. repo.Update(ctx, tx, existing)
```

## Queries SELECT con JOIN

```sql
-- GetByID, GetBySlug, List
SELECT
  p.id, p.organization_id, p.client_id, p.name, p.slug,
  p.description, p.repository_url, p.template_id,
  p.settings, p.created_at, p.updated_at, p.deleted_at,
  c.slug AS client_slug,
  c.name AS client_name
FROM projects p
LEFT JOIN clients c
  ON c.id = p.client_id
  AND c.deleted_at IS NULL
WHERE p.deleted_at IS NULL
  AND p.slug = $1; -- u otra cláusula
```

(El JOIN es "best effort": si el cliente fue archivado/soft-deleted,
`client_slug`/`client_name` salen NULL aunque `client_id` no lo sea.
Eso es correcto desde el punto de vista del operador: el cliente fue
archivado.)

## DTOs Handler REST

```go
type ProjectResponse struct {
    ID             uuid.UUID  `json:"id"`
    OrganizationID uuid.UUID  `json:"organization_id"`
    ClientID       *uuid.UUID `json:"client_id,omitempty"`
    ClientSlug     *string    `json:"client_slug,omitempty"`
    ClientName     *string    `json:"client_name,omitempty"`
    Name           string     `json:"name"`
    Slug           string     `json:"slug"`
    // ...resto previo
}

type CreateProjectRequest struct {
    Name       string  `json:"name"`
    Slug       string  `json:"slug"`
    ClientSlug *string `json:"client_slug,omitempty"`
    // ...
}

type UpdateProjectRequest struct {
    Name       *string                  `json:"name,omitempty"`
    ClientSlug presence.Optional[string] `json:"client_slug,omitempty"`
    // ...
}
```

## Filtros en List

- `?client_slug=acme-corp` → resolver → filtrar por client_id.
- `?client_slug=__none__` → filtrar por `client_id IS NULL`.
  (Nombre alternativo: `?has_client=false`. Decidir con consistencia.)
- Errores: client no existe → 422.

## MCP

Inputs añadidos en `project.create`, `project.update`, `project.list`:
```json
"client_slug": { "type": ["string", "null"], "pattern": "^[a-z0-9][a-z0-9-]{0,98}[a-z0-9]$" }
```
Outputs incluyen `client_slug` y `client_name` (nullable).

## Errores nuevos

```go
var (
    ErrClientNotFound = errors.New("project: referenced client not found in organization")
    ErrClientArchived = errors.New("project: cannot reference an archived client")
)
```

Mapeo handler/MCP:
- REST: ambos → 422 con body `{"error":"client_not_found"|"client_archived"}`.
- MCP: tool error `client_not_found` / `client_archived`.

## Coverage objetivo

- Cambios al service.Project: ≥80% en la nueva lógica.
- Tests integración nuevos: 6+ casos cubriendo escenarios del issue.md.
