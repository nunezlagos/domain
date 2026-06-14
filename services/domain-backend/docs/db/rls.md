# Row-Level Security — Domain

> issue-25.5 — migration `000028_create_rls_sensitive` + `internal/store/txctx/`

RLS es **defense-in-depth**: aunque un bug de RBAC en la capa de servicio
deje pasar una query cross-org, Postgres retorna 0 filas o rechaza el INSERT.

## Tablas con RLS activo (FORCE)

| Tabla | Política |
|-------|----------|
| `secrets` | `organization_id = current_org_id()` (USING + WITH CHECK) |
| `api_keys` | idem |
| `activity_log` | idem |
| `otp_codes` | `user_id = current_user_id()` |
| `audit_log` | SELECT con org filter; INSERT sin restricción (append-only system) |

`FORCE ROW LEVEL SECURITY` aplica las políticas incluso al owner de la tabla.

Funciones `current_org_id()` / `current_user_id()` (STABLE) leen
`app.current_org_id` / `app.current_user_id` de los session settings.

## Cómo consultar estas tablas — txctx

TODO acceso runtime a tablas RLS va envuelto en el helper:

```go
import "nunezlagos/domain/internal/store/txctx"

err := txctx.WithOrgTx(ctx, pool, orgID, func(tx pgx.Tx) error {
    rows, err := tx.Query(ctx, `SELECT ... FROM secrets WHERE ...`)
    ...
})
```

- El helper ejecuta `SELECT set_config('app.current_org_id', $1, true)`
  **dentro de la tx** (`SET LOCAL`): muere al COMMIT, seguro incluso con
  PgBouncer transaction-pooling.
- Variantes: `WithUserTx` (otp_codes), `WithOrgUserTx` (ambos).
- `uuid.Nil` es rechazado — nunca se setea contexto vacío.

Sin el helper: SELECT retorna **0 filas** e INSERT es **rechazado por
WITH CHECK** — ese es el comportamiento deseado, no un bug.

## Bypass legítimo — app_admin

El pool `pools.Auth` (user `app_admin`, BYPASSRLS) es SOLO para el path de
auth/audit donde el org aún no se conoce (ver `.claude/rules/connection-pools.md`).
Usarlo para esquivar RLS por conveniencia está prohibido.

## Tests

`internal/store/txctx/txctx_integration_test.go` (8 escenarios, testcontainers
con `SET ROLE app_user` vía AfterConnect):

- Aislamiento org en secrets / activity_log / api_keys
- Sabotaje: sin SET LOCAL → 0 rows
- Sabotaje: SELECT cross-org por id explícito → 0 rows
- Sabotaje: INSERT con organization_id ajeno → rechazado (WITH CHECK)
- SET LOCAL scoped a la tx (post-commit → 0 rows)

## Pendiente (tracked en issue-25.5)

- rls-005: linter que detecte queries a tablas RLS sin helper txctx
- rls-008: benchmark de regresión (<5%)
- Extensión a tablas Tier-1 futuras (observations particionadas, messages,
  sessions) cuando existan
