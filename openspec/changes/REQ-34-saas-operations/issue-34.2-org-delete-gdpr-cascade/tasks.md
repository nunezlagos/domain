# Tasks: issue-34.2-org-delete-gdpr-cascade

## Backend

- [ ] **T1**: Crear tabla `org_delete_log` en nueva migration
  (`migrations/000096_org_delete_log.sql`):
  - Columnas: id, organization_id (uuid, no FK), slug (text),
    actor_user_id (uuid), pre_counts (jsonb), s3_cleanup_failed
    (bool), duration_ms (int), created_at.
  - Esta tabla NO tiene FK a organizations → sobrevive al
    delete (forense).

- [ ] **T2]: Crear `internal/service/org/delete.go`:
  - `type Service struct { Pool, Audit, S3, Logger }`.
  - `DeleteOrg(ctx, orgID, actor) (*DeleteResult, error)`:
    1. Verificar existencia (si no existe → return idempotent OK).
    2. Pre-count (query agregada).
    3. INSERT en `org_delete_log` (el "initiated" entry).
    4. S3 cleanup `DeletePrefix("orgs/<id>/")` (best-effort,
       captura err).
    5. `DELETE FROM organizations WHERE id = $1` (CASCADE).
    6. UPDATE `org_delete_log SET s3_cleanup_failed, duration_ms`.
    7. Return result.

- [ ] **T3]: Helper `preCountOrgData(ctx, pool, orgID)
  ([]TableCount, error)`: query UNION ALL con conteos por tabla
  (observations, prompts, knowledge_docs, skills, agents, flows,
  flow_runs, cost_logs, audit_log, etc.).

- [ ] **T4]: `internal/api/handler/admin/orgs.go`:
  - `OrgDeleteDELETE(w, r)`:
    1. Auth check (admin role).
    2. Header check: `r.Header.Get("X-Confirm") == "true"`. Si
       no → 400 con "X-Confirm: true required".
    3. Parsear `orgID` del path.
    4. `svc.DeleteOrg(ctx, orgID, principal.UserID)`.
    5. Si idempotente (no existía) → 204 igual.
    6. Si OK → 204.
    7. Si err → 500 con detalle (sin leak de info sensible).

- [ ] **T5`: CLI `cmd/domain/org_delete.go`:
  - `domain org delete <slug> [--confirm] [--yes]`.
  - Lookup `slug → orgID` con query.
  - Si no existe → "not found" + exit 0 (idempotente).
  - Pre-count + display.
  - Sin `--confirm`/`--yes`: prompt con `Type 'DELETE <slug>' to
    confirm:`.
  - Procede.

- [ ] **T6`: Wire en `cmd/domain/main.go`:
  - Switch: `case "org": runOrgCmd(os.Args[2:])`.
  - `runOrgCmd` parsea subcommand (`delete` por ahora, puede
    extenderse con `list`, `info`, etc).

- [ ] **T7`: S3 client: el `s3client` existente
  (`internal/storage/s3/`) ya tiene `DeletePrefix` o
  equivalente. Verificar y agregar si falta. Si no hay S3
  configurado → skip cleanup (audit log marca
  `s3_configured: false`).

## Tests

- [ ] **T-unit-1]: `TestDeleteOrg_CascadesAll**` — org con data
  en 8 tablas → delete → 0 filas en cada tabla para esa org.
- [ ] **T-unit-2]: `TestDeleteOrg_InsertsDeleteLog**` — delete
  → `org_delete_log` tiene entry con pre_counts correctos.
- [ ] `TestDeleteOrg_Idempotent**` — delete 2 veces la misma
  org → 2da retorna OK sin error, sin double-delete en log.
- [ ] `TestDeleteOrg_S3BestEffort**` — S3 mockeado que falla →
  delete de DB igual procede, log tiene `s3_cleanup_failed: true`.
- [ ] `TestDeleteOrg_S3Success**` — S3 mockeado que retorna OK →
  log tiene `s3_cleanup_failed: false`.
- [ ] `TestOrgDeleteCLI_RequiresConfirm**` — `domain org delete
  acme` SIN --confirm → prompt con `DELETE acme`. Tipear
  cualquier otra cosa → abort, exit 1, NO delete.
- [ ] `TestOrgDeleteCLI_ConfirmFlag**` — `domain org delete
  acme --confirm` → skip prompt, delete directo.
- [ ] `TestOrgDeleteAPI_NoConfirmHeader**` — DELETE
  /admin/orgs/{id} SIN `X-Confirm: true` → 400.
- [ ] `TestOrgDeleteAPI_WithConfirm**` — DELETE CON
  `X-Confirm: true` → 204, org borrada.
- [ ] `TestOrgDeleteAPI_NotFound**` — DELETE org inexistente
  con X-Confirm: true → 204 (idempotente).
- [ ] `T-sabotaje`: Comentar el confirm prompt en CLI →
  `TestOrgDeleteCLI_RequiresConfirm` DEBE FALLAR → restaurar
  prompt → test verde. Documentar en commit body.
