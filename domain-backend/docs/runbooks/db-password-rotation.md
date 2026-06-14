# DB Password Rotation Runbook — issue-25.10

Policy: rotar passwords de roles Postgres cada 90 días (o ante leak sospechoso).

## Modelo

Postgres soporta **un solo password activo por role** (SCRAM). Sin downtime se
logra con rolling deploy:

1. Generar nuevo password
2. `ALTER ROLE app_user PASSWORD 'new'`
3. Actualizar K8s Secret / Secret Manager
4. Rolling deploy: pods nuevos toman el nuevo password
5. Pods viejos siguen funcionando con el password viejo HASTA su próxima
   conexión nueva (las conexiones existentes no se cierran)

> ⚠️ Postgres acepta solo el password actual. Si `password_current != ALTER ROLE
> value`, las nuevas conexiones de pods viejos fallan. Por eso el rolling debe
> ser rápido (~5min para el pool) y el connection pooler debe poder reconnect.

## Procedimiento manual

```bash
# 1. Verificar conectividad como admin
PGPASSWORD=$ADMIN_PASS psql -h $DB_HOST -U app_admin -d domain -c "SELECT 1"

# 2. Generar + aplicar nuevo password
NEW_PASS=$(domain rotate-db-password --role app_user)
# El comando:
#   - genera 32 bytes random base64url
#   - ejecuta ALTER ROLE app_user PASSWORD '<dollar-quoted>'
#   - imprime el nuevo password a stdout

# 3. Actualizar el Secret Manager (ejemplo AWS Secrets Manager)
aws secretsmanager update-secret \
  --secret-id prod/domain/app_user_password \
  --secret-string "$NEW_PASS"

# 4. Forzar reload del ESO en K8s (External Secrets Operator)
kubectl annotate externalsecret app-user-password \
  force-sync=$(date +%s) --overwrite

# 5. Rolling restart de pods Domain
kubectl rollout restart deployment/domain-server

# 6. Verificar healthcheck OK en todos los pods nuevos
kubectl rollout status deployment/domain-server --timeout=5m

# 7. Limpiar variable local
unset NEW_PASS
```

## Audit log

Tras cada rotación manual, registrar en `audit_log` (manual o vía endpoint admin):
- `actor` = SRE que ejecutó
- `action` = `db.password.rotated_manual`
- `entity_type` = `db_role`
- `entity_id` = `app_user`

## Cron K8s (futuro)

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: db-password-rotation
spec:
  schedule: "0 3 1 */3 *"  # Cada 3 meses, día 1 a las 3am UTC
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: rotator
            image: domain-server:latest
            command:
            - /bin/sh
            - -c
            - |
              NEW=$(domain rotate-db-password --role app_user)
              aws secretsmanager update-secret \
                --secret-id prod/domain/app_user_password \
                --secret-string "$NEW"
              kubectl rollout restart deployment/domain-server
          serviceAccountName: db-rotator
          restartPolicy: OnFailure
```

## Rollback

Si una rotación falla mid-way (ej. Secret actualizado pero ALTER ROLE no se
aplicó):

1. Re-ejecutar `domain rotate-db-password` para forzar nuevo password sync
2. Si no se puede recuperar, restaurar password viejo desde Secret Manager
   versionado y re-ejecutar `ALTER ROLE` con ese valor manualmente (psql)
3. Notificar al equipo de Security antes de cualquier rollback

## Caveat PgBouncer

Si PgBouncer está delante de Postgres, la rotación REQUIERE actualizar también
su `userlist.txt` (o re-generar via `SHOW USERS;` query):

```bash
echo "\"app_user\" \"SCRAM-SHA-256$....\"" > /etc/pgbouncer/userlist.txt
pgbouncer -R  # online reload
```

Si no se actualiza, las conexiones nuevas de pods fallan en PgBouncer aunque
Postgres acepte el password.
