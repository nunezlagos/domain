# Tasks: issue-34.1-self-service-export-zip

## Backend

- [ ] **T1**: Crear `internal/api/handler/export.go` con
  `ExportGET(w, r)`:
  - Auth check (Bearer o session).
  - `orgID := principal.OrganizationID`, `userID := principal.UserID`.
  - Set headers: `Content-Type: application/zip`,
    `Content-Disposition: attachment; filename=...`.
  - Crear `zip.NewWriter(w)`, defer close.
  - Para cada tabla: crear file en zip, gzip wrap,
    `pgx.CopyTo` con SELECT que retorna JSON.
  - metadata.json al final (escrito primero lógicamente, pero
    el ZIP order no importa).
  - Audit log post-success.

- [ ] **T2**: Crear `internal/api/handler/export_zipper.go`:
  - `StreamOrgExport(ctx, pool, orgID, userID, zw *zip.Writer)
    error`.
  - Lista de tablas hardcoded: observations, prompts,
    knowledge_docs, skills, agents, flows, flow_runs, audit_log.
  - Helper `streamTableToZip(ctx, pool, tableName, orgID, userID,
    zw) error`:
    - Crea file en zip: `<table>.jsonl.gz`.
    - `gz := gzip.NewWriter(fw)`.
    - Query: `SELECT row_to_json(t) FROM <table> t WHERE
      organization_id = $1 AND deleted_at IS NULL` (con
      excepciones para tablas que no tienen organization_id
      directo).
    - `pgx.CopyTo(gz, pool, query, orgID)`.
    - `gz.Close()`, `fw.Close()`.

- [ ] **T3**: Helper `BuildMetadata(orgID, userID) []byte` que
  arma el JSON de metadata (domain version, timestamps, schema
  version, lista de tablas).

- [ ] **T4**: `internal/api/handler/export_audit.go`:
  - `RecordExportAudit(ctx, pool, userID, orgID, metadata map[string]any) error`.
  - INSERT en `audit_log` con `action='export'`.

- [ ] **T5]: Wire en `cmd/domain/main.go`:
  - `mux.Handle("/api/v1/export", authMW(...).Wrap(exportHandler))`.
  - Allowlist en rate limit middleware (33.1): skip `/api/v1/export`.
  - Allowlist en write timeout del http.Server: el handler
    setea `w.Header().Set("Connection", "close")` y deshabilita
    timeout internamente (con context con timeout propio).

- [ ] **T6**: Agregar a `config.Config`:
  - `ExportMaxBytes int64` (default 50*1024*1024*1024 = 50GB).
  - `ExportTimeout time.Duration` (default 30*time.Minute).
  - Si el export excede MaxBytes, loggear y abortar stream
    (best-effort, hard to enforce mid-stream).

- [ ] **T7`: CLI `cmd/domain/export.go`:
  - `domain export [--base-url URL] [--api-key KEY] [--output FILE]`.
  - Default `--output` es stdout (`> my-backup.zip`).
  - Hace `http.Get($baseURL/api/v1/export)`, streama body al
    output.

- [ ] **T8`: Tests de integración: helper
  `setupExportTestData(orgID, n int)` que inserta N observations,
  M prompts, etc. para que el test verifique contenido del ZIP.

## Tests

- [ ] **T-unit-1**: `TestStreamOrgExport_IncludesAllTables**` —
  org con data en todas las tablas → ZIP tiene los 8 archivos
  esperados + metadata.json.
- [ ] **T-unit-2**: `TestStreamOrgExport_OnlyOrgData**` — 2 orgs,
  user de A hace export → ZIP solo tiene data de A (verificar
  contando rows de cada .jsonl.gz descomprimido).
- [ ] **T-unit-3**: `TestStreamOrgExport_EmptyOrg**` — org sin data
  → ZIP tiene metadata.json + 8 archivos vacíos (gzip válido
  de 0 bytes).
- [ ] **T-unit-4**: `TestStreamOrgExport_AuditLogOnlyCaller**` — el
  audit_log.jsonl.gz tiene SOLO eventos del user caller, no
  de otros users de la org.
- [ ] **T-e2e-1`: `TestE2E_ExportLargeOrg_Streams**` — insertar
  100K observations (~50MB JSON) → export → ZIP completo,
  memory del server <200MB durante el export (medir con
  `runtime.MemStats`).
- [ ] **T-e2e-2`: `TestE2E_ExportAudited**` — export → audit_log
  tiene entry con `action=export`, `resource=org/<id>`, metadata
  con bytes_streamed.
- [ ] **T-e2e-3`: `TestE2E_ExportCLI**` — `domain export` contra
  un server mockeado → stdout tiene un ZIP válido.
- [ ] **T-sabotaje`: Comentar `WHERE organization_id = $1` en
  una de las queries de tabla (sabotaje: no filtra) → test
  unit-2 DEBE FALLAR (data de B aparece en A) → restaurar
  filtro → test verde. Documentar en commit body.
