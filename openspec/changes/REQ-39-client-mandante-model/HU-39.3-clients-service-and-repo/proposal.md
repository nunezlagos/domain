# Proposal: HU-39.3-clients-service-and-repo

## Intención

Implementar la capa de servicio + repositorio para `clients`, replicando
el patrón ya consolidado en `internal/service/project/` (interface
`Repository`, struct `Service`, implementación pgx `pg_repository.go`)
y honrando los principios:

- Una sola fuente de validación (no duplicar reglas en handler/MCP).
- Errores tipados domain-level (no `fmt.Errorf` opacos).
- Uso de `txctx.WithOrgTx` para garantizar RLS activa en cada operación.

## Scope

**Incluye:**
- `internal/service/client/service.go` — struct `Service` + métodos:
  `Create`, `GetByID`, `GetBySlug`, `List`, `Update`, `Archive`, `Restore`.
- `internal/service/client/repository.go` — interface `Repository` que
  abstrae el acceso a DB para facilitar tests con fakes.
- `internal/service/client/pg_repository.go` — implementación pgx que
  ejecuta SQL contra `clients` dentro de `txctx.WithOrgTx`.
- Tipos: `Client` (entidad), `CreateInput`, `UpdateInput`, `ListFilter`,
  errores (`ErrSlugConflict`, `ErrNotFound`, `ErrInvalidInput`).
- Validación de slug (`^[a-z0-9][a-z0-9-]{1,98}[a-z0-9]$` o similar) y de
  status (uno de `active`/`inactive`/`archived`).
- Tests unitarios con repo fake (sin Postgres).
- Test de integración contra Postgres real (`service_integration_test.go`)
  cubriendo crear, conflicto de slug, list aislada por org, archive,
  cross-tenant ENOENT.

**No incluye:**
- Handler REST → HU-39.4.
- Tool MCP → HU-39.5.
- Wiring del service en el bootstrap del servidor → HU-39.4 lo activa
  cuando registra el handler.
- Migraciones (ya existen en 39.1 / 39.2).
- Modificación de `projects` service → HU-39.6.

## Enfoque técnico

1. **Estructura de archivos** alineada con `internal/service/project/`:
   ```
   internal/service/client/
     ├── service.go              (Service + métodos públicos)
     ├── repository.go           (interface Repository + tipos input)
     ├── pg_repository.go        (pgx impl)
     └── service_integration_test.go
   ```
2. **Wrappers `WithOrgTx`**: cada método público del Service obtiene
   `orgID` del context (vía `auth.OrgFromContext(ctx)` o helper
   equivalente) y envuelve la operación con `txctx.WithOrgTx`. El repo
   recibe `pgx.Tx` (no `*pgxpool.Pool`).
3. **Mapeo de errores Postgres**:
   - `23505` (unique_violation) en (organization_id, slug) → `ErrSlugConflict`.
   - `23514` (check_violation) en status → `ErrInvalidInput`.
   - `pgx.ErrNoRows` → `ErrNotFound`.
4. **Validación previa al SQL** para evitar round-trip innecesario:
   - Slug regex.
   - Name no vacío.
   - Email malformado (si no vacío) → ErrInvalidInput.
   - Status fuera del set → ErrInvalidInput.
5. **Soft delete via Archive**: `UPDATE SET status='archived', deleted_at=NOW()
   WHERE id=$1 AND deleted_at IS NULL`.
6. **List con paginación cursor**: reutilizar `api/cursor` helpers ya
   existentes; por simplicidad esta HU implementa el SELECT base ordenado
   por `created_at DESC, id DESC` y soporta `Limit` (default 50, max 200).

## Riesgos

- **Olvidar `WithOrgTx`**: el repo nunca debe ejecutar SQL fuera de
  `WithOrgTx`. Mitigación: la interface `Repository` recibe `pgx.Tx`, no
  `*pgxpool.Pool`. Tests aseguran que cualquier método nuevo lo respete.
- **Cursor desincronizado con RLS**: como RLS filtra por org, la
  paginación basada en (created_at, id) es estable per-org. Si el código
  futuro hace cross-org queries (admin), debe usar otro repo (no este).
- **Doble fuente de validación slug**: si HU-39.4 también valida slug
  en el handler, se duplica regla. Mitigación: el service es la única
  fuente; handler solo decodifica JSON.
- **`Restore` puede colisionar con slug ocupado**: si un cliente está
  archivado y otro nuevo tomó su slug, restore debe fallar con
  ErrSlugConflict. La UNIQUE constraint lo enforce automáticamente.

## Testing

- `internal/service/client/service_test.go` (unit, con fake repo):
  cubre validaciones de input, mapeo de errores, lógica de archive.
- `internal/service/client/service_integration_test.go` (integration,
  Postgres real):
  - Crea 2 orgs vía fixture, valida que List per-org no fuga.
  - Verifica conflict de slug retorna ErrSlugConflict.
  - Verifica que sin SET LOCAL (caller fuera de WithOrgTx) las queries
    devuelven 0 rows (defensa contra bypass).
  - Verifica archive/restore y que listar con `IncludeArchived` los
    muestra.
- Coverage target del package: ≥80% (consistente con resto).
