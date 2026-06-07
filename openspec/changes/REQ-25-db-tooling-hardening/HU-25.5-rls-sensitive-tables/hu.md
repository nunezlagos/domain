# HU-25.5-rls-sensitive-tables

**Origen:** `REQ-25-db-tooling-hardening`
**Prioridad tentativa:** alta
**Tipo:** feature + hardening

## Historia de usuario

**Como** plataforma multi-tenant
**Quiero** Row-Level Security activa a nivel Postgres en tablas sensibles
**Para** que un bug en RBAC de la app NO permita cross-org data leak

## Tablas con RLS

| tabla | scope | razón |
|-------|-------|-------|
| secrets | organization_id | datos cifrados de la org |
| billing/subscriptions | organization_id | facturación |
| stripe_events_processed | organization_id | events Stripe por org |
| audit_log | organization_id | compliance |
| sessions | user_id + organization_id | sesiones auth |
| otp_codes | user_id | códigos one-time |
| idempotency_records | organization_id | request cache |
| notification_deliveries | organization_id | mensajes salientes |
| custom_roles | organization_id | RBAC |
| outbound_webhook_subscriptions | organization_id | webhooks |
| agent_memory_kv (scope=organization) | organization_id | memoria scoped |
| export_jobs | user_id | GDPR export |

## Criterios de aceptación

### Escenario 1: RLS activa con policy por org

```gherkin
Dado que la tabla `secrets` tiene `ROW LEVEL SECURITY ENABLED`
Y existe policy `secrets_org_isolation` USING `organization_id = current_setting('app.current_org')::uuid`
Cuando el app_user hace SELECT * FROM secrets sin SET app.current_org
Entonces devuelve 0 rows (policy denies)
Cuando antes hace `SET LOCAL app.current_org = 'org-X-uuid'`
Entonces solo devuelve rows de org X
```

### Escenario 2: SET LOCAL en cada transacción

```gherkin
Dado que la app abre tx
Cuando arranca tx
Entonces hace `SET LOCAL app.current_org = $1` y `SET LOCAL app.current_user = $2`
Y el valor vive solo durante la tx (importante para PgBouncer transaction-pooling)
Y al COMMIT/ROLLBACK se descarta
```

### Escenario 3: Bypass para admin/superuser

```gherkin
Dado que un job batch (cleanup, drift check) corre como `app_admin` role
Cuando hace SELECT * FROM secrets sin app.current_org
Entonces ve todas las rows (bypass policy)
Y app_admin tiene grant BYPASSRLS
Y app_user NO tiene BYPASSRLS
```

### Escenario 4: Bug RBAC simulado bloqueado

```gherkin
Dado que un bug hipotético permite a la app intentar `SELECT * FROM secrets WHERE id = 'cross-org-leak'`
Y la app no setea app.current_org correctamente o seteo es otra org
Cuando Postgres ejecuta
Entonces RLS bloquea la fila → 0 rows
Y el bug NO causa leak
```

### Escenario 5: Performance acceptable

```gherkin
Dado que policy usa `organization_id = current_setting(...)::uuid`
Cuando query corre
Entonces query planner incluye predicate junto a index `(organization_id, ...)` existente
Y overhead <5% vs no-RLS
```

### Escenario 6: Policies en migración

```gherkin
Dado que las policies se aplican via migrations versionadas
Cuando migration N corre
Entonces se ejecuta `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` + CREATE POLICY
Y down hace DISABLE + DROP POLICY
```

## Análisis breve

- **Qué pide:** RLS en 12 tablas + helper SET LOCAL + bypass admin + tests adversariales
- **Esfuerzo:** M
- **Riesgos:** olvidarse SET LOCAL = 0 rows confunde; performance regression; migration compleja
