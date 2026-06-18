# Single-org — estado actual

> REQ-21.5 (surface collapse) + REQ-21.6 (schema decommission) — vigente desde 2026-06-08.

## TL;DR

Domain corre como **single-org** desde REQ-21.5. La tabla `organizations` sigue
existiendo pero es **un dato huérfano** — el código no la gestiona, no la usa como FK
operativa, y se va a dropear en Fase C de REQ-21.6. Toda la machinery multi-tenant
(RLS por org, GUC `app.current_org_id`, threading de `organization_id` en queries,
FKs cruzadas) está apagada o en proceso de apagado.

## Lo que cambió en REQ-21.5 (surface collapse, hecho)

- `internal/service/org` (servicio de gestión de orgs) **eliminado**.
- Endpoint backend `POST /organizations` (create) y `DELETE /organizations/:id` (delete)
  **eliminados**. Solo quedan `GET /organizations/:id` (lectura) y `PATCH .../:id`
  (ajuste de settings/limits).
- SDK Go/Python/TypeScript: `Organization.Create`/`Delete` eliminados (REQ-21.5 commit
  fbc8ba6). `OrganizationsResource`/`organizations` reducido a `Get` + `ListMembers`.
- API key auth: `Issue`/`Rotate` ya no reciben `orgID` (la firma se conserva por
  compat con callers pero el param es `_ = orgID`). El org se deriva del user vía
  `JOIN users.organization_id` en `apikey.Resolve`.
- Enrollment tokens: tabla `org_enrollment_tokens` ahora es single-row global (sin
  scoping por org). `Rotate`/`Revoke`/`Enroll` operan sobre la única row activa.
- Plans: existen pero solo como tiers internos de control de uso. `organizations.plan_id`
  FK queda hasta Fase C (esa migración la dropea).
- Invitations: **eliminadas completamente** (issue-21.2 removida en REQ-21.5).

## Lo que cambió en REQ-21.6 Fase A (org schema decommission)

- `000132_disable_org_rls`: RLS org **DESHABILITADA** en 19 tablas. Policies inertes
  (reversibles). Defense-in-depth perdida aceptada en org-isolation (single-org no
  necesita defensa cross-org).
- Middleware: ya no setea `app.current_org_id` (GUC queda vacío). Sigue seteando
  `app.current_user_id` para `otp_codes` RLS.

## Lo que cambió en REQ-21.6 Fase B (per-consumer cleanup, hecho commit bcc5196)

- `000135 cost_alerts_sent_org_nullable`: `organization_id` nullable,
  `UNIQUE(alert_date)` (era `UNIQUE(organization_id, alert_date)`).
- `000136 org_cost_alert_thresholds_pk_swap`: PK `organization_id` → `id BIGSERIAL`.
- `000137 org_flow_config_pk_swap`: mismo swap.
- `000138 usage_counters_pk_swap`: PK compuesta `(organization_id, period_start)` →
  `id BIGSERIAL` + `UNIQUE(period_start)`.
- `000139 org_enrollment_tokens_drop_org_fk`: FK a `organizations(id)` + UNIQUE
  INDEX partial `(organization_id) WHERE revoked_at IS NULL` → UNIQUE INDEX partial
  `((TRUE)) WHERE revoked_at IS NULL` (singleton global activo).
- 4 integration tests nuevos en `internal/service/{usagealerts,flow,billing,enrollment}/`.

## Lo que cambió en REQ-21.6 Fase D (periferia, parcial)

- SDK Go: `Organization` struct eliminado de `types.go`. `organizations.go` borrado.
  `Project.OrganizationID` y `Observation.OrganizationID` removidos del wire format.
- SDK Python: `Organization` model + `organizations_id` fields eliminados.
  `OrganizationsResource` removido de `resources.py`.
- SDK TypeScript: idem (`Organization` interface + fields).
- Tests SDK: fixtures actualizados (sin `organization_id` en JSON responses).
- Seeds/fixtures/tests (~189 refs con `organization_id`): **pendientes**. La columna
  sigue existiendo hasta Fase C; los tests siguen funcionando con ella.
- Docs: `docs/db/rls.md` actualizado para reflejar Fase A.

## Lo que queda (Fase C — destructiva, irreversible)

- `000140+`: `DROP CONSTRAINT organizations_*_fkey` en todas las FKs que apuntan a
  `organizations(id)`. Crítico antes de poder dropear la tabla.
- `000141+`: `DROP COLUMN organization_id` en las 54 tablas que aún la tienen
  (preservando filas — las que tienen FK a `organizations` ya deben estar dropeadas).
- `000142+`: `DROP FUNCTION current_org_id()` + `DROP TRIGGER projects_client_same_org_check`.
- `000143+`: `DROP TABLE organizations` + satellites (`org_delete_log` ya está, faltan
  invitations/usage_counters/plans/org_* pero la mayoría ya no existen en código).

**Pre-requisito absoluto**: backup verificado con `pgBackRest` (ver
`docs/runbooks/restore.md`) + dry-run local en DB efímera (testcontainers) para
validar conteo de filas pre/post por tabla antes de tocar VPS.

## Cómo verificar el estado actual

```bash
# ¿RLS org activa?
psql -c "SELECT relname, relrowsecurity FROM pg_class WHERE relname IN ('secrets','api_keys','audit_log','otp_codes');"

# ¿Tabla organizations existe aún?
psql -c "SELECT count(*) FROM organizations;"

# ¿Migraciones Fase B aplicadas?
psql -c "SELECT version FROM schema_migrations WHERE version >= '135' ORDER BY version;"

# ¿GUC org seteado por el middleware?
psql -c "SHOW app.current_org_id;"  # debería ser vacío tras Fase A
```

## Riesgos vivos

- **Race entre deploys**: si se deploya el binario Fase A sin haber aplicado la migración
  `000132`, el código intenta usar RLS que ya no está en DB → queries fallan. Workaround:
  siempre migrar ANTES de deployar el binario.
- **Tests legacy**: `txctx_integration_test.go` tiene tests de aislamiento org marcados
  `t.Skip`. Si se reactiva RLS accidentalmente (rollback), esos tests deben volver a pasar.
- **Backups viejos**: snapshots pre-Fase-A tienen `app.current_org_id` seteado en queries
  lentas de pg_stat_statements. No afecta correctness pero infla métricas.
