# Design: issue-34.2-org-delete-gdpr-cascade

## Contexto

GDPR Art. 17 (Right to be forgotten) + casos de cuentas
abandonadas: el operador necesita poder borrar una org completa.
El schema YA TIENE `ON DELETE CASCADE` en las FKs (verificado en
migrations), así que el delete es 1 SQL statement. El valor del
issue es el wrapping operacional:

1. Audit log ANTES y DESPUÉS.
2. Confirm doble (evitar accidents).
3. S3 cleanup.
4. Idempotencia.
5. Endpoint CLI + API.

## Decisión arquitectónica

**Estrategia:** comando CLI + endpoint API, ambos wrappeando
`DELETE FROM organizations WHERE id = $1` + S3 cleanup + audit.

1. **Función core en service layer:**
   ```go
   // internal/service/org/delete.go
   func (s *Service) DeleteOrg(ctx context.Context, orgID uuid.UUID) (*DeleteResult, error) {
     // 1. Audit log ANTES
     s.audit.Record(ctx, audit.Event{
       Actor: "admin",
       Action: "org.delete_initiated",
       Resource: fmt.Sprintf("org/%s", orgID),
       Metadata: preCountMetadata(ctx, s.pool, orgID),
     })

     // 2. S3 cleanup (best-effort)
     s3Err := s.s3.DeletePrefix(ctx, fmt.Sprintf("orgs/%s/", orgID))
     s3CleanupFailed := s3Err != nil

     // 3. CASCADE delete
     result, err := s.pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
     if err != nil { return nil, err }

     // 4. Audit log DESPUÉS
     s.audit.Record(ctx, audit.Event{
       Actor: "admin",
       Action: "org.delete_completed",
       Resource: fmt.Sprintf("org/%s", orgID),
       Metadata: postDeleteMetadata(result, s3CleanupFailed),
     })

     return &DeleteResult{
       OrgID: orgID,
       RowsDeleted: result.RowsAffected(),
       S3CleanupFailed: s3CleanupFailed,
     }, nil
   }
   ```

2. **Pre-count metadata:** query agregada que cuenta filas por
   tabla ANTES del delete:
   ```sql
   SELECT 'observations' AS table, COUNT(*) FROM observations WHERE organization_id = $1
   UNION ALL
   SELECT 'agents', COUNT(*) FROM agents WHERE organization_id = $1
   ...
   ```
   Esto da el impacto esperado para el confirm prompt.

3. **Confirm doble (CLI):**
   ```go
   fmt.Printf("ABOUT TO DELETE org '%s' (id=%s):\n", slug, id)
   for _, count := range preCounts {
     fmt.Printf("  - %d %s\n", count.N, count.Table)
   }
   fmt.Printf("Proceed? Type 'DELETE %s' to confirm:\n", slug)
   var input string
   fmt.Scanln(&input)
   if input != fmt.Sprintf("DELETE %s", slug) {
     return errors.New("aborted")
   }
   ```
   Si `--confirm` o `--yes` se pasa → skip prompt.

4. **Endpoint API:**
   - `DELETE /api/v1/admin/orgs/{id}`.
   - Auth: admin role.
   - Header required: `X-Confirm: true` (defensa contra CSRF/click).
   - Sin header → 400 con "X-Confirm: true header required".
   - Con header → procede.
   - 204 No Content en éxito.
   - 404 si la org no existe (idempotente: el segundo delete
     también retorna 204 — ver #5).

5. **Idempotencia:**
   - Verificar existencia antes del confirm prompt (CLI) o del
     delete (API).
   - Si no existe: warning + exit 0 (CLI) o 204 (API).

6. **S3 cleanup best-effort:**
   - Si S3 falla: loggear warning, continuar con delete de DB.
   - Audit log marca `s3_cleanup_failed: true`.
   - El admin puede correr un script de reconciliación
     post-mortem.

7. **Permisos:** el delete requiere `DOMAIN_DATABASE_AUTH_URL`
   (BYPASSRLS) en CLI. La API usa el pool normal (con RLS)
   pero el handler es admin-only.

8. **Audit retention:** el delete borra TODOS los audit_log
   entries de la org, incluso los >90 días. La retention
   policy (issue-23.2) NO se respeta para deletes GDPR
   explícitos. Documentado.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Soft delete (marcar `deleted_at`, dejar rows) | El user quiere delete real. Soft delete + cron purge es una opción pero más complejo. GDPR requiere borrado real. |
| B | Anonimizar en vez de borrar | El cliente tiene derecho a que se borren SUS datos. Anonimizar no es lo que pide GDPR. |
| C | Borrar tabla por tabla con orden específico | CASCADE lo hace en 1 statement. Más queries = más riesgo de inconsistencia. |
| D | Hard delete + restore window de 30 días | Más complejo (tabla `deleted_orgs_archive`). Out of scope. |
| E | Confirm via email (enviar código al admin) | El admin que corre el comando YA está autenticado. Doble confirm textual es suficiente. |

## Por qué CASCADE + audit + confirm doble gana

- **CASCADE:** ya está en el schema. Reusar.
- **Audit antes/después:** forense completa. Si falla, sabemos
  qué se intentó.
- **Confirm doble:** UX simple pero efectiva. El admin tiene
  que tipear `DELETE <slug>` textualmente.
- **S3 best-effort:** priorizamos GDPR (borrar DB) sobre
  attachments. Si S3 falla, se reconcilia después.

## Detalle de implementación

- `internal/service/org/delete.go` con la función core.
- `internal/api/handler/admin/orgs.go` con `OrgDeleteDELETE`.
- `cmd/domain/org_delete.go` con el CLI.
- Wire en `cmd/domain/main.go` switch:
  `case "org": runOrgCmd(os.Args[2:])`.
- Helper `preCountOrgData(ctx, pool, orgID) ([]TableCount, error)`.

## Riesgos

- **R1:** El admin borra la org equivocada. **Mitigación:** doble
  confirm textual + pre-count (impacto visible). Si aún así
  pasa, no hay restore (es decisión del admin).
- **R2:** El CASCADE delete puede tardar minutos con muchas
  filas. **Mitigación:** timeout configurable (default 5min),
  warning si tarda >2min.
- **R3:** S3 cleanup con muchos attachments es lento. **Aceptable:**
  es best-effort, no bloquea.
- **R4:** Audit log ANTES del delete también se borra (por
  CASCADE). **Decisión:** guardar el "initiated" event en
  una tabla separada `org_delete_log` que NO tiene FK a
  organizations. Solo guarda: timestamp, actor, org_id, slug,
  pre-counts. Es la prueba forense definitiva.
