# Design: issue-25.10-db-secrets-rotation

## Flow

```
1. Generate new_password (crypto/rand 32 bytes b64)
2. ALTER ROLE app_user PASSWORD 'new_password'
3. Update Secret/AWS-SM with new_password
4. (ESO syncs in <60s OR explicit kubectl rollout restart deployment/domain)
5. Wait for rollout complete (all pods on new password) — kubectl wait
6. Verify: no pod still using old password (check connection logs via pgaudit)
7. Update PgBouncer userlist + reload via PGBOUNCER ADMIN
8. Audit log "db.password.rotated_success"
On failure:
  9. ALTER ROLE app_user PASSWORD old_password
  10. Audit "db.password.rotation_rolledback"
```

## Schedule (staggered)

```yaml
# k8s CronJobs
rotate-app-user:      "0 4 1 */3 *"     # day 1 every 3 months
rotate-app-admin:     "0 4 8 */3 *"     # day 8
rotate-app-readonly:  "0 4 15 */3 *"    # day 15
rotate-app-migrator:  "0 4 22 */3 *"    # day 22
```

## CLI command

```bash
domain-mcp rotate-db-password --role app_user --confirm
```

## TDD plan

1. Manual rotation zero-downtime (app pods siguen respondiendo durante el rollout)
2. Cron scheduled cada role
3. Rollback en failure
4. ESO/AWS-SM sync visible
5. PgBouncer reload after rotate
6. Sabotaje: rollout fail at 50% → rollback automático
