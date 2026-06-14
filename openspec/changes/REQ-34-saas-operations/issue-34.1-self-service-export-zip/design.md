# Design: issue-34.1-self-service-export-zip

## Contexto

GDPR Art. 20 (Right to data portability) requiere que el usuario
pueda obtener una copia de SUS datos en formato estructurado y de
uso común. Para domain, eso es: JSON Lines (estándar para
streaming) + gzip (compresión) + ZIP (un solo archivo portable).

Adicionalmente, casos de uso legítimos: backup personal, migrar a
otro sistema, auditorías. El endpoint debe ser seguro (solo ve
SU org), performante (no agota memoria), y trazable (audit log).

## Decisión arquitectónica

**Estrategia:** streaming zip writer + queries por org +
audit log post-export.

1. **Endpoint:** `GET /api/v1/export` (NO `POST` — es read-only).
   - Auth: Bearer OR session.
   - Sin rate limit (operación larga legítima, allowlist en 33.1).
   - `WriteTimeout` deshabilitado para esta ruta (config en
     `cmd/domain/main.go`).

2. **Schema del ZIP:**
   ```
   domain-export-<slug>-<YYYYMMDD>.zip
     metadata.json           ← header info
     observations.jsonl.gz   ← todas las observations
     prompts.jsonl.gz
     knowledge_docs.jsonl.gz
     skills.jsonl.gz
     agents.jsonl.gz
     flows.jsonl.gz
     flow_runs.jsonl.gz
     audit_log.jsonl.gz     ← solo del user caller
   ```

3. **Formato JSON Lines:** cada línea es un JSON object
   independiente. Termina con `\n`. Gzip por archivo (no el ZIP
   entero, para que se pueda inspeccionar individualmente).

4. **Streaming con `archive/zip` + `pgx.CopyTo`:**
   ```go
   w.Header().Set("Content-Type", "application/zip")
   w.Header().Set("Content-Disposition", "attachment; filename=...")
   zw := zip.NewWriter(w)
   defer zw.Close()

   for _, table := range []string{"observations", "prompts", ...} {
     fw, _ := zw.CreateHeader(&zip.FileHeader{
       Name: table + ".jsonl.gz",
       Method: zip.Deflate,
     })
     gz := gzip.NewWriter(fw)
     pgx.CopyTo(gz, pool, fmt.Sprintf(`
       SELECT json_build_object(...) FROM %s
       WHERE organization_id = $1 AND deleted_at IS NULL
     `, table), orgID)
     gz.Close()
   }
   ```
   `pgx.CopyTo` streamea rows directo al writer sin cargarlas en
   memoria.

5. **Filtro de org:** TODAS las queries llevan
   `WHERE organization_id = $principal.OrganizationID`. Para
   `audit_log`, el filtro es
   `actor_user_id = $principal.UserID OR organization_id = $orgID`
   (decisión: incluir eventos del user en otras orgs, no — solo
   del user caller).

6. **metadata.json schema:**
   ```json
   {
     "domain_version": "0.x.y",
     "exported_at": "RFC3339",
     "exported_by_user_id": "uuid",
     "exported_by_email": "user@org.com",
     "organization": {id, slug, name, created_at},
     "schema_version": "1",
     "tables_exported": ["observations", "prompts", ...],
     "format": "jsonl.gz"
   }
   ```

7. **Audit log entry** post-streaming exitoso:
   - `INSERT INTO audit_log (actor_user_id, organization_id,
     action, resource, metadata, occurred_at) VALUES ($1, $2,
     'export', 'org/<id>', $3, NOW())`.
   - Metadata: `{bytes_streamed, duration_ms, row_counts:
     {observations: 100, ...}}`.

8. **Manejo de errores mid-stream:** si una query falla, el
   stream se corta. El cliente recibe un ZIP truncado (no es un
   ZIP válido). Loggear el error con detalle. El audit log entry
   no se inserta (porque no fue exitoso).

9. **Límite de tamaño (defensa):** si el export excede 50GB,
   cortar el stream y loggear "export too large, suggest chunked
   export" (feature futura). 50GB es suficiente para el 99% de
   los casos.

10. **Comando CLI `domain export`** que envuelve el endpoint:
    `domain export > my-backup.zip`. Útil para el user que
    prefiere CLI sobre HTTP.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | CSV en vez de JSONL | CSV pierde tipos (timestamps, JSONB). JSONL es estándar. |
| B | Comprimir el ZIP entero (no por archivo) | Menos flexible: no se puede inspeccionar un archivo individual sin descomprimir todo. |
| C | Almacenar el ZIP en S3 y dar URL firmada | Más complejo, requiere cleanup, da URL que puede ser compartida. Stream directo es más simple y seguro. |
| D | Soportar import (no solo export) | El user explícitamente dijo "no endpoint de import". El ZIP es para que el user decida. |
| E | Exportar TODO el dominio (no per-org) | Es lo que hace el backup global (34.4). El self-service es per-org. |

## Por qué streaming zip + pgx.CopyTo gana

- **Memory-bounded:** no importa si la org tiene 10M rows, el
  server usa ~50MB constantes.
- **Time-to-first-byte:** el cliente ve el ZIP downloading
  inmediatamente, no espera 10 minutos para que el server arme
  todo.
- **Formato portable:** ZIP + JSONL.gz es entendido por
  cualquier herramienta (jq, gzip, unzip).
- **Auditable:** el audit log entry queda con metadata
  completa.

## Detalle de implementación

- `internal/api/handler/export.go` con `ExportGET(w, r)`.
- `internal/api/handler/export_zipper.go` con helper
  `StreamOrgExport(ctx, pool, orgID, userID, zw *zip.Writer)
  error`.
- `internal/api/handler/export_audit.go` con
  `RecordExportAudit(ctx, pool, userID, orgID, metadata) error`.
- Wire en `cmd/domain/main.go` con allowlist en rate limit
  (modificar el middleware de 33.1 para excluir /api/v1/export).
- Config: agregar `ExportMaxBytes int64` (default 50GB),
  `ExportTimeout time.Duration` (default 30min) a `config.Config`.
- CLI: `cmd/domain/export.go` que hace
  `http.Get($baseURL/api/v1/export)`.

## Riesgos

- **R1:** El usuario hace export y luego pide delete (34.2) sin
  haber descargado bien el ZIP. **Aceptable:** el export es
  síncrono, el cliente sabe cuándo terminó (status 200 + body
  completo). El audit log marca el éxito.
- **R2:** Connection drop mid-stream pierde progreso. **Aceptable:**
  el user re-corre. No es crítico (no es backup del server, es
  del user).
- **R3:** `pgx.CopyTo` no soporta JSON marshaling custom. **Mitigación:**
  usar `pgx.CopyTo` con un wrapper que serializa cada row a
  JSON antes de escribir.
