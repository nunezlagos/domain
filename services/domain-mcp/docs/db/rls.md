# Row-Level Security — Domain

> issue-25.5 (migración `000028_create_rls_sensitive`) + REQ-21.6 (org schema decommission).

## Estado actual (2026-06-18)

### Fase A de REQ-21.6 ejecutada — RLS org DESHABILITADA

Migración `000132_disable_org_rls` aplica `DISABLE ROW LEVEL SECURITY` en las 19 tablas
org-scoped (NO `otp_codes` — su RLS es user-isolation). Las policies `*_org_isolation`
siguen definidas pero son **inertes**: cualquier query pasa la policy vacía. Reversible
(`down.sql` hace `ENABLE ROW LEVEL SECURITY FORCE`). El drop definitivo de las policies +
función `current_org_id()` queda para la **Fase C** destructiva.

### Single-org (REQ-21.5)

REQ-21.5 colapsó el surface multi-tenant. La única `organizations` row sigue existiendo
(mientras la Fase C no se ejecute) pero el código no la gestiona. `users.organization_id`
queda huérfano: el backend lo deriva de la API key vía JOIN en `apikey.Resolve` y no se
usa como FK operativa. Las organizaciones NO se crean/eliminan vía API desde REQ-21.5.

### Defense in depth: perdida aceptada en org-isolation, conservada en user-isolation

`otp_codes` mantiene RLS con `current_user_id()` — sigue siendo defense-in-depth real para
atajos cross-user. La pérdida de defense-in-depth en las tablas org-scoped es aceptada
porque single-org == todo el dataset es la misma org (no hay cross-org que defender).

## Tablas con RLS activo (parcial, post-Fase-A)

| Tabla | Política | Estado |
|-------|----------|--------|
| `secrets` | `organization_id = current_org_id()` (USING + WITH CHECK) | **DESHABILITADA** (Fase A) |
| `api_keys` | idem | **DESHABILITADA** |
| `activity_log` | idem | **DESHABILITADA** |
| `otp_codes` | `user_id = current_user_id()` | **ACTIVA** (user-isolation) |
| `audit_log` | SELECT con org filter; INSERT sin restricción | **DESHABILITADA** |

`FORCE ROW LEVEL SECURITY` ya no se aplica a las tablas org-scoped (la policy queda
inerte tras `DISABLE ROW LEVEL SECURITY`). Las tablas marcadas como "DESHABILITADA"
siguen usando `app_user` (NOBYPASSRLS) pero la policy es vacía, así que retorna todas
las filas visibles al user.

## Funciones helpers

- `current_user_id()` (STABLE): lee `app.current_user_id` de session settings. Sigue
  usándose en `otp_codes` RLS (única tabla RLS-on tras Fase A).
- `current_org_id()` (STABLE): lee `app.current_org_id` de session settings. Sigue
  existiendo (no drop) pero el código ya no la invoca — el middleware dejó de ejecutar
  `set_config('app.current_org_id', ...)` en Fase A. Se elimina en Fase C junto con la
  tabla `organizations`.

## Cómo consultar tablas — txctx

`internal/store/txctx/` expone `WithUserTx` (para `otp_codes`) y ya no usa
`WithOrgTx`/`WithOrgUserTx` activamente en runtime (el org ID se deriva vía JOIN en el
store de apikey, no vía GUC). El helper sigue compilando para no romper importadores
históricos pero no se usa fuera de tests legacy.

```go
// Único path activo:
err := txctx.WithUserTx(ctx, pool, userID, func(tx pgx.Tx) error {
    rows, err := tx.Query(ctx, `SELECT ... FROM otp_codes WHERE ...`)
    ...
})
```

## Bypass legítimo — app_admin

El pool `pools.Auth` (user `app_admin`, BYPASSRLS) es SOLO para el path de auth/audit
donde el user aún no se conoce (ver `.claude/rules/connection-pools.md`). Usarlo para
esquivar RLS por conveniencia está prohibido.

## Tests

`internal/store/txctx/txctx_integration_test.go` (8 escenarios, testcontainers
con `SET ROLE app_user` vía AfterConnect):

- Aislamiento user en otp_codes (sigue activo)
- Sabotaje: sin SET LOCAL → 0 rows en tablas con RLS activa (otp_codes)
- Tests de aislamiento org están marcados `t.Skip` con nota "DESHABILITADA Fase A".
  Se reactivarán cuando se restaure la policy (rollback Fase A) o se eliminan cuando
  la columna `organization_id` se dropee en Fase C.

## Pendiente (Fase C de REQ-21.6, destructiva)

- `000140_*`: dropear FK constraints desde satellites hacia `organizations(id)`.
  Ya cubierto por las migraciones 000135-000139 (Fase B).
- `000141_*`: `DROP COLUMN organization_id` en satellites (5 tablas: cost_alerts_sent,
  org_cost_alert_thresholds, org_flow_config, usage_counters, org_enrollment_tokens).
- `000142_*`: `DROP FUNCTION current_org_id()` + `DROP TRIGGER projects_client_same_org_check`.
- `000143_*`: `DROP TABLE organizations` (root) + satellites.

Cada paso requiere backup verificado (pgBackRest, ver `docs/runbooks/restore.md`) y
dry-run local antes de tocar VPS.
