# Design: issue-34.4-backup-nightly-b2

## Contexto

El VPS puede perderse (ransomware, proveedor caído, etc).
Necesitamos un backup OFF-SITE del Postgres entero. Backblaze B2
es la opción costo-efectiva: S3-compatible, $0.005/GB/mes, 10GB
gratis. Para una DB de 1GB, son $0.005/mes.

El flow canónico: cron nightly → pg_dump → gzip → upload a B2 →
verificar → cleanup local → audit log. Adicional: restore test
semanal para verificar que el backup es restaurable (no
corrupto).

## Decisión arquitectónica

**Estrategia:** script bash standalone + cron entry + job de
restore test semanal.

1. **Script `deploy/backup/backup.sh`:**
   ```bash
   #!/bin/bash
   set -euo pipefail

   DATE=$(date -u +%Y%m%d)
   BACKUP_FILE="/tmp/db-${DATE}.dump"
   COMPRESSED="${BACKUP_FILE}.gz"

   # 1. pg_dump (custom format, compressed natively)
   pg_dump --format=custom --no-owner --file="${BACKUP_FILE}" "${DOMAIN_DATABASE_URL}"

   # 2. gzip adicional (opcional, custom format ya está comprimido)
   gzip "${BACKUP_FILE}"

   # 3. Upload a B2
   aws s3 cp "${COMPRESSED}" \
     "s3://${BACKUP_B2_BUCKET}/db/${DATE}.dump.gz" \
     --endpoint "${BACKUP_B2_ENDPOINT}"

   # 4. Verificar size
   LOCAL_SIZE=$(stat -c%s "${COMPRESSED}")
   REMOTE_SIZE=$(aws s3api head-object --bucket "${BACKUP_B2_BUCKET}" --key "db/${DATE}.dump.gz" --endpoint "${BACKUP_B2_ENDPOINT}" --query ContentLength --output text)
   if [ "$LOCAL_SIZE" != "$REMOTE_SIZE" ]; then
     echo "size mismatch: local=$LOCAL_SIZE remote=$REMOTE_SIZE" >&2
     exit 1
   fi

   # 5. Cleanup local
   rm -f "${COMPRESSED}"

   # 6. Retention: borrar >30 días
   aws s3 ls "s3://${BACKUP_B2_BUCKET}/db/" --endpoint "${BACKUP_B2_ENDPOINT}" \
     | while read -r line; do
       FILE_DATE=$(echo "$line" | awk '{print $4}' | grep -oP '\d{8}')
       if [ -n "$FILE_DATE" ]; then
         AGE_DAYS=$(( ($(date +%s) - $(date -d "$FILE_DATE" +%s)) / 86400 ))
         if [ "$AGE_DAYS" -gt 30 ]; then
           aws s3 rm "s3://${BACKUP_B2_BUCKET}/db/${FILE_DATE}.dump.gz" --endpoint "${BACKUP_B2_ENDPOINT}"
         fi
       fi
     done
   ```

2. **Config (env vars):**
   - `BACKUP_ENABLED=true` (kill switch).
   - `BACKUP_B2_BUCKET=domain-backups`.
   - `BACKUP_B2_ENDPOINT=https://s3.us-west-004.backblazeb2.com`.
   - `BACKUP_B2_KEY_ID=...`.
   - `BACKUP_B2_APP_KEY=...`.
   - `BACKUP_RETENTION_DAYS=30` (default).
   - `BACKUP_ADMIN_EMAIL=ops@tudominio.com` (para alertas).

3. **Cron entry** (`/etc/cron.d/domain-backup`):
   ```
   0 3 * * * root /opt/domain/deploy/backup/backup.sh >> /var/log/domain-backup.log 2>&1
   ```
   `0 3 * * *` = 3am diario. Asume VPS en UTC.

4. **Detección de 2 fallas consecutivas:**
   - Después del script, escribir un marker file en B2:
     `state/last-success.txt` con la fecha del último success.
   - El script chequea: `if (today - last_success_date) > 1 day`
     AND script falló hoy → enviar email.
   - Implementación simple: `aws s3 cp` + `aws s3 cp state/`.
   - O alternativa: tabla `backup_state` en Postgres (pero eso
     es chicken-and-egg si la DB se pierde).

5. **Restore test semanal** (`deploy/backup/test-restore.sh`):
   - Corre los domingos 4am.
   - Descarga último backup de B2.
   - Crea DB `domain_restore_test` (distinta de producción).
   - `pg_restore` en esa DB.
   - Smoke tests: `SELECT COUNT(*) FROM organizations`, etc.
   - DROP DATABASE al final.
   - Si falla: email al admin.
   - Cron entry: `0 4 * * 0 root /opt/domain/deploy/backup/test-restore.sh`.

6. **Documentación** (`docs/runbooks/restore-from-b2.md`):
   - Paso a paso del restore manual.
   - Comando exactos.
   - Edge cases (qué hacer si el restore da errores).

7. **Audit log entry** (opcional, si el server puede escribir al
   momento del backup):
   - `action=backup.completed, metadata={date, size_bytes,
     duration_ms, b2_key}`.
   - PROBLEMA: si el server está caído, no puede escribir.
   - Solución: el script escribe a un file en B2 (`state/backup.log`),
     y el server lo lee al boot para actualizar el audit log.

8. **Encryption at rest:** B2 tiene encryption built-in (AES-256).
   El dump NO se encripta adicionalmente (overhead > benefit). Si
   el operador quiere, puede usar `gpg --symmetric` antes de
   upload (feature flag).

9. **Notification email:** reusar el `mail.Mailer` de 34.3. Si
   SMTP no está configurado → log warning en vez de email.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Backup incremental (WAL archiving) | Más complejo, requiere setup continuo. El nightly full es suficiente para el caso. |
| B | Backup a S3 AWS (en vez de B2) | Más caro ($0.023/GB vs $0.005/GB). B2 es la opción costo-efectiva. |
| C | Backup in-process (Go code) en vez de bash | pgx tiene `pg_dump` como sub-comando; bash es más portable y debuggeable. El script es simple. |
| D | Backup cada hora | Overkill para nuestro caso. Diario es suficiente. |
| E | Backup de archivos de MinIO attachments | Out of scope: el user puede agregar después. El DB es la fuente de verdad. |

## Por qué bash script + cron + B2 gana

- **Simplicidad:** 1 script de ~50 líneas + 1 cron entry.
- **Portable:** bash está en cualquier Linux. `pg_dump` y `aws`
  son binarios standard.
- **Debuggeable:** logs en `/var/log/domain-backup.log`. El
  operador puede revisar fácilmente.
- **Testeable:** el restore test semanal valida el flow
  end-to-end sin esperar a una catastrophe.
- **Costo-efectivo:** B2 es 5x más barato que S3 standard.

## Detalle de implementación

- `deploy/backup/backup.sh` con el flow.
- `deploy/backup/test-restore.sh` con el restore semanal.
- `deploy/backup/backup.cron` con las 2 líneas crontab.
- `docs/runbooks/restore-from-b2.md` con el manual.
- `scripts/test-backup-e2e.sh` para correr el flow completo
  contra un Postgres de test (CI o manual).
- Config en el `.env` del VPS: documentar las env vars.

## Riesgos

- **R1:** B2 tiene outage. **Aceptable:** el siguiente día
  intenta de nuevo. Si 2+ días falla → email.
- **R2:** VPS clock drift. **Mitigación:** usar `date -u` (UTC)
  siempre, no local time.
- **R3:** El script se rompe por cambio de versión de
  `pg_dump`. **Mitigación:** el restore test detecta esto
  (falla al restaurar). El operador puede rollback.
- **R4:** Backups crecen >10GB (free tier de B2). **Aceptable:**
  el operador paga. Documentar sizing esperado.
