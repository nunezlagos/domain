# Capa de datos con sqlc

Patrón para escribir el acceso a datos en los services de `internal/service/*`.
Las queries SQL viven en un archivo `.sql` versionado y `sqlc` genera código Go
type-safe a partir de ellas. Reemplaza el SQL inline + `rows.Scan` posicional.

## Por qué

La capa de datos venía a dos velocidades: ~9 services con repository dedicado y
~36 con queries SQL embebidas en `service.go` y escaneadas a mano. Eso significa
queries no identificadas (imposibles de auditar como conjunto) y `Scan` posicional
frágil: agregar una columna y olvidar un `&campo` compila pero rompe en runtime.

sqlc resuelve ambos: las queries quedan en un catálogo `.sql` con nombre, y el
mapeo columna→campo lo genera la herramienta (no se puede desalinear).

El piloto es `internal/service/issue/`. Migrar los 44 services restantes es trabajo
mecánico pendiente — este doc es la referencia para hacerlo.

## Estructura por service

```
internal/service/<nombre>/
├── sqlc.yaml          # config (apunta a las migraciones reales)
├── sql/query.sql      # catálogo de queries con anotaciones -- name:
├── <nombre>db/        # GENERADO por sqlc — no editar a mano
│   ├── db.go
│   ├── models.go
│   └── query.sql.go
└── service.go         # lógica de dominio: validación, audit, mapeo
```

## sqlc.yaml

El `schema` apunta al directorio **real** de migraciones. sqlc entiende el formato
golang-migrate (`*.up.sql` / `*.down.sql`) e ignora los `down` solo — cero schema
duplicado que mantener. Probado contra las 154 migraciones reales (con functions,
triggers y RLS) sin error.

Los `overrides` son obligatorios para que los tipos generados coincidan con los del
dominio (si no, sqlc emite `pgtype.UUID`, `pgtype.Text`, etc.):

```yaml
overrides:
  - { db_type: "uuid", go_type: "github.com/google/uuid.UUID" }
  - { db_type: "uuid", nullable: true, go_type: { import: "github.com/google/uuid", type: "UUID", pointer: true } }
  - { db_type: "timestamptz", go_type: "time.Time" }
  - { db_type: "text", nullable: true, go_type: { type: "string", pointer: true } }
```

`omit_unused_structs: true` evita que `models.go` emita un struct por cada tabla del
schema (sin él pesa ~56 KB).

## Escribir queries

Anotación `-- name: NombreQuery :one|:many|:exec|:execrows` antes de cada query.

**Orden de columnas = reutilización de modelo.** Si el `SELECT`/`RETURNING` lista las
columnas en el mismo orden físico que la tabla, sqlc reutiliza el struct de tabla
(`Issue`) en vez de generar un `XxxRow` por query. Vale la pena alinear.

**Filtros opcionales** (el caso del `List` con `WHERE` dinámico): usar `sqlc.narg`
en vez de armar el `WHERE` a mano con `fmt.Sprintf`:

```sql
WHERE (sqlc.narg('status')::text IS NULL OR us.status = sqlc.narg('status')::text)
```

Genera un param `*string` nullable; pasás `nil` para "sin filtro".

## Transacciones y RLS — IMPORTANTE

El `*Queries` generado opera sobre cualquier `DBTX` (lo satisface `*pgxpool.Pool` y
`pgx.Tx`):

- Operación simple → `issuedb.New(s.Pool)`.
- Transacción → `issuedb.New(tx)` (o `q.WithTx(tx)`).

**Tablas con RLS FORCE** (`organizations`, `projects`, `users`, `observations`,
`sessions`, `clients`): las queries DEBEN correr dentro de la tx que tiene
`SET LOCAL app.current_org_id` activo. Para esos services hay que pasar la tx
org-scopeada a `issuedb.New(tx)`, **nunca** el pool crudo — si no, `app_user`
(NOBYPASSRLS) ve 0 filas. Las tablas de `issue` no tienen RLS, por eso el piloto usa
`s.Pool` directo.

## Mapeo a dominio

El struct generado (`issuedb.Issue`) es el modelo de **persistencia**. El struct
público del service (`Issue`, con JSON tags y campos derivados como `Scenarios`) es
el de **dominio**. Mapear con funciones chicas (`toIssue`, `toScenario`). Esta
separación es deliberada: la API pública no queda atada a la forma de la tabla.

## Regenerar

Tras editar `sql/query.sql`:

```bash
make sqlc            # regenera todos los paquetes con sqlc.yaml
# o por paquete:
cd internal/service/issue && go generate ./...
```

`make sqlc` usa una versión pineada de sqlc vía `go run` (no requiere instalación
global ni ensucia go.mod).
