# Connection Pools — Domain

Domain usa **dos pools tipados** por separación de privilegios DB. No es optimización;
es defense-in-depth. Cada handler/service declara explícitamente qué pool necesita.

## Los dos pools

### `pools.App` — user `app_user` (NOBYPASSRLS)

Para queries runtime ordinarias. Si la tabla tiene RLS activado (ver
`migrations/000028_create_rls_sensitive.up.sql`), el caller **DEBE** envolver
las queries en `txctx.WithOrgTx(pool, orgID, fn)` para establecer
`app.current_org_id` antes de la query. Sin esto, RLS retorna 0 rows o rechaza
INSERT.

**Tablas en este pool sin envolver**: organizations, users, projects,
observations, prompts, knowledge_docs, skills, agents, flows — todas las que
**no** tienen RLS habilitada. Estos services validan org_id en `WHERE` SQL.

### `pools.Auth` — user `app_admin` (BYPASSRLS)

Para queries del path de autenticación y auditoría donde el org_id aún no
se conoce o cruza orgs por diseño:

- `apikey.PGStore.Resolve` — lookup api_keys por prefix antes de saber org_id
- `apikey.PGStore.Issue`   — INSERT api_keys post-verify-otp
- `otp.Service.Request`    — lookup users por email/RUT cross-org
- `audit.PGRecorder.Record` — system events pueden tener org_id NULL

**Reglas duras**:
- No usar AuthPool para queries de runtime ordinarias
- No usar AuthPool para "evitar RLS por conveniencia" — eso es exactamente
  el bug que RLS previene
- Si una nueva query necesita AuthPool, agregar entrada en lista de arriba +
  justificar en code review

## En producción

DSN diferentes por user:

```bash
DOMAIN_DATABASE_URL=postgres://app_user:***@db:5432/domain?sslmode=verify-full
DOMAIN_DATABASE_AUTH_URL=postgres://app_admin:***@db:5432/domain?sslmode=verify-full
```

Si `DOMAIN_DATABASE_AUTH_URL` no está seteado, el cmd/domain server registra
una warning y reutiliza el App pool. Esto solo es aceptable en development
local single-user; en staging/prod CI rechaza el config (issue-25.6 futura).

## En tests

`db.OpenWithRoleOverride(ctx, singleDSN, "app_user", "app_admin")` crea ambos
pools sobre el mismo container usando `SET ROLE` via `pgxpool.Config.AfterConnect`.
Esto es funcionalmente equivalente a dos DSN distintas pero usa el container
único de testcontainers.

## Anti-patterns prohibidos

- ❌ `service.Pool` recibe AppPool para una query que SELECT cruza orgs sin RLS
   helper — esto leakea data cross-tenant
- ❌ Pasar AuthPool a services de domain (org, project, observation, invite)
- ❌ Usar AuthPool en handler runtime "porque RLS molesta" — usar txctx.WithOrgTx
- ❌ Único pool sin user dedicado por rol — viola separación

## Detección en CI

El linter issue-25.13 podría extenderse para verificar que `*PGStore.Pool` se
asigna desde `pools.Auth` SOLO en `internal/auth/*` y `internal/audit/*`.
Pending.
