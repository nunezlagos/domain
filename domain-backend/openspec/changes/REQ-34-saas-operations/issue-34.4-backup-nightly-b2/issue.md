# issue-34.4-backup-nightly-b2

**Origen:** `REQ-34-saas-operations`
**Prioridad tentativa:** media
**Tipo:** feature (operational/insurance)

## Historia de usuario

**Como** operador del VPS en producción
**Quiero** un cron nightly que hace `pg_dump` del Postgres entero y lo sube a Backblaze B2 (S3-compatible, ~$0.005/GB/mes)
**Para** tener un backup global en caso de catástrofe (VPS se pierde, ransomware, etc)

## Criterios de aceptación

### Escenario 1: Backup nightly corre y se sube

```gherkin
Dado que el cron `0 3 * * *` se dispara (3am diario)
Y `BACKUP_ENABLED=true` + `BACKUP_B2_BUCKET=domain-backups` + `BACKUP_B2_KEY_ID=...` + `BACKUP_B2_APP_KEY=...` están en env
Cuando el job corre
Entonces:
  1. `pg_dump --format=custom --no-owner $DOMAIN_DATABASE_URL > /tmp/db-$(date +%Y%m%d).dump`
  2. Comprimir con gzip → /tmp/db-$(date +%Y%m%d).dump.gz
  3. `aws s3 cp /tmp/db-...gz s3://$BACKUP_B2_BUCKET/db/$(date +%Y%m%d).dump.gz --endpoint $BACKUP_B2_ENDPOINT`
  4. Verificar que el size local y remoto coinciden (md5)
  5. Borrar /tmp/db-...gz
  6. Loggear success con duration + size
Y exit 0
Y audit log entry con action=backup.completed, metadata={size_bytes, duration_ms, b2_key}
```

### Escenario 2: Retention 30 días

```gherkin
Dado que hoy hay 45 backups en B2
Y la retention config es 30
Cuando corre el cron
Entonces el script de retention borra los 15 más viejos
Y deja los 30 más recientes
Y la operación es atómica (no borrar el de hoy antes de subir el nuevo)
```

### Escenario 3: Notificación si falla 2 noches seguidas

```gherkin
Dado que el backup falló ayer (exit !=0)
Y el backup falla hoy también
Cuando el job termina hoy
Entonces:
  1. Detecta "fallback_count >= 1" (de un file marker en S3 o en una tabla)
  2. Envía email al admin: "[domain] backup failed 2 nights in a row — check /var/log/domain-backup.log"
  3. NO envía email si solo falló 1 noche (puede ser transient)
```

### Escenario 4: Restore documentado paso a paso

```gherkin
Dado que el VPS se perdió y tengo un backup .dump.gz en B2
Cuando quiero restaurar
Entonces el procedimiento documentado en `docs/runbooks/restore-from-b2.md`:
  1. `aws s3 cp s3://$BUCKET/db/20260612.dump.gz /tmp/ --endpoint $ENDPOINT`
  2. `gunzip /tmp/20260612.dump.gz`
  3. `pg_restore --no-owner --dbname=$NEW_DB_URL /tmp/20260612.dump`
  4. Verificar: `psql $NEW_DB_URL -c "SELECT COUNT(*) FROM organizations"`
Y cada paso está probado (hay un script `scripts/test-restore.sh` que valida el flujo contra un Postgres de test)
```

### Escenario 5: Sabotaje — backup NO sube a B2 (solo queda local)

```gherkin
Dado que el código tiene un bug (sabotaje) que solo hace pg_dump y gzip
Y NO sube a B2
Cuando corre el cron
Entonces el archivo queda en /tmp pero se borra al final
Y no hay nada en B2
Y el audit log miente (dice "uploaded" pero no fue)
Y el test e2e que assserta "el archivo existe en B2 post-backup"
DEBE FALLAR
Cuando restauro la lógica de upload
Entonces el test verde
```

### Escenario 6: Edge case — Postgres down al momento del backup

```gherkin
Dado que Postgres no está accesible (server down, network)
Cuando el cron corre
Entonces `pg_dump` falla con "connection refused"
Y el script retorna exit !=0
Y NO se intenta el upload a B2 (no hay qué subir)
Y se loggea el error
Y el día siguiente, si vuelve a fallar, se envía email de alerta
```

### Escenario 7: Edge case — B2 credentials inválidas

```gherkin
Dado que `BACKUP_B2_KEY_ID` es inválido
Cuando el cron intenta subir
Entonces `aws s3 cp` falla con "InvalidAccessKeyId"
Y el script retorna exit !=0
Y el log captura el error de AWS
Y se loggea WARNING: "check B2 credentials in $BACKUP_B2_KEY_ID"
```

### Escenario 8: Verificación end-to-end (restore test) semanal

```gherkin
Dado que un job adicional corre los domingos a las 4am
Cuando ejecuta
Entonces descarga el último backup de B2
Y lo restaura en una DB de test (NOMBRE distinto, e.g.
`domain_restore_test`)
Y corre smoke tests: SELECT COUNT(*) FROM organizations, etc.
Y al final, dropea la DB de test
Y si el restore falla, envía email: "restore from B2 failed — last good backup is <date>"
```

## Notas

- Backblaze B2 es S3-compatible. Se puede usar `aws s3 cp` con
  `--endpoint https://s3.<region>.backblazeb2.com`.
- El formato `pg_dump --format=custom` es comprimido nativo
  (mejor que `--format=plain` + gzip externo).
- La verificación md5 entre local y remoto es opcional pero
  tranquilidad.
- Retention en B2 vía lifecycle policy (configurada una vez en
  el bucket) o via script de cleanup post-upload.
