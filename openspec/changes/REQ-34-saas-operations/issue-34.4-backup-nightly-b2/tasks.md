# Tasks: issue-34.4-backup-nightly-b2

## Backend

- [ ] **T1]: Crear `deploy/backup/backup.sh`:
  - Recibe env vars (DATABASE_URL, B2_BUCKET, B2_ENDPOINT,
    B2_KEY_ID, B2_APP_KEY, RETENTION_DAYS).
  - `pg_dump --format=custom --no-owner --file=$tmp $DATABASE_URL`.
  - `gzip $tmp`.
  - `aws s3 cp $tmp s3://$BUCKET/db/$DATE.dump.gz --endpoint $ENDPOINT`.
  - Verificar size con `aws s3api head-object`.
  - `rm $tmp`.
  - Retention: listar archivos viejos, borrar >RETENTION_DAYS.
  - `set -euo pipefail` al inicio.
  - Logs a stdout (que van a /var/log/domain-backup.log via
    cron).

- [ ] **T2]: Crear `deploy/backup/backup.cron`:
  ```
  0 3 * * * root /opt/domain/deploy/backup/backup.sh >> /var/log/domain-backup.log 2>&1
  0 4 * * 0 root /opt/domain/deploy/backup/test-restore.sh >> /var/log/domain-restore-test.log 2>&1
  ```

- [ ] **T3]: Setup de credenciales B2 en el VPS:
  - `aws configure` con B2 key_id + app_key.
  - O env vars en `/opt/domain/.env` (más portable).
  - Documentar en `deploy/contabo/README.md` (issue-31.3).

- [ ] **T4]: Detección de 2 fallas consecutivas:
  - Al final del script (success o fail), escribir/actualizar
    `s3://$BUCKET/state/last-success.txt` (solo en success).
  - Al inicio del script, leer ese file.
  - Si no existe o es de hace >1 día Y el script falla hoy →
    enviar email.
  - Email subject: "[domain] backup failed 2 nights in a row".

- [ ] **T5]: Email helper `deploy/backup/notify-failure.sh`:
  - Usa `mail` command o `curl` al endpoint del mailer de 34.3.
  - Si el server tiene un endpoint admin `POST /admin/notify`,
    usarlo (más limpio que SMTP desde bash).

- [ ] **T6]: Crear `deploy/backup/test-restore.sh`:
  - Descarga último backup de B2.
  - Crea DB temporal `domain_restore_test`.
  - `pg_restore --no-owner --dbname=$RESTORE_TEST_URL $dump`.
  - Smoke tests: `psql -c "SELECT COUNT(*) FROM organizations"`,
    `SELECT COUNT(*) FROM users`, etc.
  - DROP DATABASE al final.
  - Si falla, envía email.
  - Logs a /var/log/domain-restore-test.log.

- [ ] **T7]: Documentación `docs/runbooks/restore-from-b2.md`:
  - Pre-requisitos (aws cli, psql client, acceso a B2).
  - Paso 1: `aws s3 cp s3://$BUCKET/db/<date>.dump.gz /tmp/`.
  - Paso 2: `gunzip /tmp/<date>.dump.gz`.
  - Paso 3: `pg_restore --no-owner --dbname=$NEW_DB /tmp/<date>.dump`.
  - Paso 4: smoke tests.
  - Edge cases (qué hacer si pg_restore da errores de role).

- [ ] **T8]: Documentar env vars en `.env.example`:
  ```
  # BACKUP_ENABLED=true
  # BACKUP_B2_BUCKET=domain-backups
  # BACKUP_B2_ENDPOINT=https://s3.us-west-004.backblazeb2.com
  # BACKUP_B2_KEY_ID=your-key-id
  # BACKUP_B2_APP_KEY=your-app-key
  # BACKUP_RETENTION_DAYS=30
  # BACKUP_ADMIN_EMAIL=ops@tudominio.com
  ```

- [ ] **T9]: Script de test local `scripts/test-backup-e2e.sh`:
  - Para CI o manual.
  - Levanta Postgres en docker, crea schema, corre backup, corre
    restore, verifica counts.
  - NO usa B2 real — usa MinIO local (S3-compatible).

## Tests

- [ ] `TestBackup_PgDumpOK**` — Postgres de test con schema
  cargado → backup.sh → archivo .dump.gz existe localmente y en
  MinIO.
- [ ] `TestBackup_SizeMatches**` — backup → comparar md5 local
  vs MinIO → match.
- [ ] `TestBackup_Retention30Days**` — crear 35 backups dummy
  en MinIO → backup.sh → solo quedan 30 (los 5 más viejos
  borrados).
- [ ] `TestBackup_NotifiesOnSecondFailure**` — mock SMTP →
  forzar 2 fallas → email enviado con subject correcto.
- [ ] `TestBackup_NoNotifyOnFirstFailure**` — forzar 1 falla →
  NO email (la alerta es para 2+).
- [ ] `TestBackup_PostgresDown**` — Postgres no accesible →
  pg_dump falla → script retorna exit !=0 → NO intenta upload.
- [ ] `TestRestore_EndToEnd**` — backup completo → restore en
  DB nueva → counts match (organizations, users, etc).
- [ ] `TestRestoreTest_WeeklyCron**` — verificar que el script
  test-restore.sh corre, restaura, valida, dropea.
- [ ] `T-sabotaje`: Comentar la línea de `aws s3 cp` (sabotaje:
  backup queda local, NO sube) → test que assserta "archivo en
  MinIO post-backup" DEBE FALLAR → restaurar upload → test
  verde. Documentar en commit body.
