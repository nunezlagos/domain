# Tasks: issue-34.5-audit-log-multi-tenant

## Backend

- [ ] **T1]: Crear `migrations/000097_audit_log_org.sql`:
  - `ALTER TABLE audit_log ADD COLUMN origin_org_id UUID REFERENCES organizations(id)`.
  - `CREATE INDEX idx_audit_log_org_time ON audit_log(origin_org_id, occurred_at DESC)`.
  - `CREATE INDEX idx_audit_log_org_action ON audit_log(origin_org_id, action) WHERE origin_org_id IS NOT NULL`.
  - Backfill en batches: intentar derivar del `resource` (regex
    `^org/([0-9a-f-]{36})/`).

- [ ] **T2]: Refactor `internal/audit/recorder.go`:
  - `Record(ctx, event Event)` ya existe.
  - Agregar `OriginOrgID *uuid.UUID` al struct `Event`.
  - Si el caller no lo setea, el recorder lo deriva del
    principal del context (post-auth).
  - Si no hay principal ni org_id explícito → NULL (eventos
    del system, e.g. cron jobs).

- [ ] **T3]: Auditar call-sites:
  - Buscar todos los `audit.Record(` en el código.
  - Agregar `OriginOrgID: &principal.OrganizationID` donde
    aplique.
  - Verificar que el 100% de los eventos tenant-scoped tienen
    org_id.

- [ ] **T4]: `internal/api/handler/admin/audit.go`:
  - `AuditListGET(w, r)`:
    1. Auth check (admin o super_admin).
    2. Parse query params: org_id, since, until, action,
       resource, cursor, limit.
    3. Validación: si role=admin, org_id == principal.OrganizationID
       (sino 403).
    4. Si role=super_admin, org_id opcional (cross-org).
    5. Decode cursor si presente.
    6. Query SQL con filtros.
    7. Encode next_cursor si hay más.
    8. JSON response.

- [ ] **T5]: `internal/api/handler/admin/audit_cursor.go`:
  - `type Cursor struct { TS time.Time; ID uuid.UUID }`.
  - `EncodeCursor(c Cursor) string` → base64(json(c)).
  - `DecodeCursor(s string) (Cursor, error)`.

- [ ] **T6]: Filtro action con wildcard:
  - `action=*.delete` → SQL `action LIKE '%delete'`.
  - Validar input: máximo 100 chars, sin `;` (defensa
    injection, aunque parameterized queries ya previenen).
  - Si no tiene wildcard, exact match.

- [ ] **T7]: Wire en `cmd/domain/main.go` con auth admin.

- [ ] **T8]: OpenAPI annotations (issue 32.3) para
  `AuditListGET`. Tag "Admin".

- [ ] **T9]: Backfill de entries existentes:
  - Script Go standalone `cmd/backfill-audit-org/main.go` o
    similar.
  - Itera en batches de 10K, UPDATE el origin_org_id derivado
    del `resource` o del `actor_user_id.organization_id`.
  - Loggea progreso y summary.
  - Idempotente: corre múltiples veces, no duplica.

## Tests

- [ ] `TestAuditList_PaginatesCorrectly**` — 100 entries →
  limit=20 → 5 pages de 20 cada una, no overlap, no skip.
- [ ] `TestAuditList_FilterByOrg**` — 2 orgs con 50 entries
  cada una → query con `org_id=A` → 50 entries, todas de A.
- [ ] `TestAuditList_FilterByActionWildcard**` — mix de
  `observation.create` y `observation.delete` → query
  `action=*.delete` → solo deletes.
- [ ] `TestAuditList_FilterByResource**` — query
  `resource=observation/abc` → solo entries de esa resource.
- [ ] `TestAuditList_AdminOnlySeesOwnOrg**` — admin de org A
  query `org_id=B` → 403.
- [ ] `TestAuditList_SuperAdminSeesAll**` — super_admin sin
  org_id → recibe entries de todas las orgs.
- [ ] `TestAuditList_CursorStability**` — insertar N entries
  nuevas entre 2 paginaciones → la 2da page NO omite entries
  nuevos (vs offset que sí).
- [ ] `TestAuditList_Performance**` — 1M entries → query con
  limit=50 → <200ms.
- [ ] `TestAuditRecorder_OrgIDFromPrincipal**` — evento sin
  OriginOrgID explícito + principal en context → el recorder
  popla desde principal.
- [ ] `TestAuditRecorder_OrgIDFromEvent**` — evento CON
  OriginOrgID explícito → el recorder usa ese (no override).
- [ ] `T-sabotaje`: Comentar la validación "admin solo ve SU org"
  en `AuditListGET` (sabotaje: no valida) → test
  `TestAuditList_AdminOnlySeesOwnOrg` DEBE FALLAR (admin A ve
  data de B) → restaurar validación → test verde. Documentar
  en commit body.
