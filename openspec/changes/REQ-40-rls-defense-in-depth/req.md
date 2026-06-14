# REQ-40 — RLS Defense-in-Depth (cierre del gap multi-tenant)

> **Origen**: sesión 2026-06-14. Auditoría del modelo multi-tenant detectó
> que `projects`, `users` y `organizations` **NO** tienen RLS activa, aunque
> sí la tienen tablas adyacentes (`secrets`, `audit_log`, `otp_codes`,
> `activity_log`, `api_keys`, `observations`, `sessions`). Esto significa
> que un bug en el código de aplicación que olvide `WHERE organization_id=...`
> **expone datos cross-tenant** sobre las 3 tablas centrales del modelo.
> Esta REQ cierra el gap activando RLS + FORCE sobre las 3 tablas
> faltantes, replicando el patrón ya validado en migración 000028 / 000085.

## Contexto

Auditoría del estado actual (post-REQ-25, REQ-28, REQ-36):

| Tabla | RLS | FORCE | Observación |
|-------|-----|-------|-------------|
| `secrets` | ✅ | ✅ | desde migración 000028 |
| `audit_log` | ✅ | ✅ | desde 000028 |
| `otp_codes` | ✅ | ✅ | desde 000028 |
| `activity_log` | ✅ | ✅ | desde 000028 |
| `api_keys` | ✅ | ✅ | desde 000028 |
| `observations` | ✅ | ✅ | desde 000085 |
| `sessions` | ✅ | ✅ | desde 000085 |
| `clients` | ✅ | ✅ | desde 000099 (REQ-39) |
| **`projects`** | ❌ | ❌ | **GAP — REQ-40** |
| **`users`** | ❌ | ❌ | **GAP — REQ-40** |
| **`organizations`** | ❌ | ❌ | **GAP — REQ-40** |

Riesgo concreto: cualquier handler o tool MCP que arme una query sobre
`projects`/`users`/`organizations` sin filtrar explícitamente por
`organization_id` retorna filas de TODOS los tenants. Hoy el código sí
filtra (no se ha detectado leak), pero **un bug futuro o un patch
descuidado expone datos cross-org sin red de seguridad**. RLS es exactly
defense-in-depth para este caso.

## Restricciones de diseño

1. **Patrón idéntico al ya validado**: usar `current_org_id()` función
   ya creada en 000028, `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` +
   `FORCE ROW LEVEL SECURITY`, policy `*_org_isolation` con USING +
   WITH CHECK por `organization_id = current_org_id()`. Sin inventar
   patrones nuevos.
2. **No-breaking**: el código actual que usa `txctx.WithOrgTx` ya setea
   `SET LOCAL app.current_org_id`. Las queries seguirán funcionando. El
   código que NO use `WithOrgTx` empezará a ver 0 rows — eso es
   exactamente la intención (forzar al code path correcto).
3. **`organizations` requiere policy especial**: la tabla raíz, por
   definición, NO tiene `organization_id` (es la id misma). La policy
   debe ser `id = current_org_id()`. Sin esto, un usuario podría listar
   todas las orgs del sistema.
4. **`users` con scope per-org**: idéntico patrón `organization_id =
   current_org_id()`. Cada org solo ve sus usuarios.
5. **`projects` con scope per-org**: idéntico patrón `organization_id =
   current_org_id()`. La columna `client_id` (de REQ-39) NO entra en la
   policy — el aislamiento se hace por org, no por cliente.
6. **Re-grants explícitos**: cada migración debe re-emitir GRANTs sobre
   las tablas afectadas (por la misma razón que 000028, 000085, 000099 —
   `ALTER DEFAULT PRIVILEGES` de 000025 no cubre todos los caminos).
7. **3 migraciones separadas**: una tabla por migración para minimizar
   blast radius y permitir revert granular. Numeradas correlativamente.
8. **Test obligatorio cross-tenant**: por cada tabla, un test que valida
   que `SELECT * FROM tabla` sin `SET LOCAL` retorna 0 rows, y con
   `SET LOCAL` correcto retorna solo filas del tenant.

## HUs

| HU | Slug | Esfuerzo | Archivos tocados | Wave |
|----|------|----------|------------------|------|
| 40.1 | `rls-projects` | S | `internal/migrate/migrations/000101_rls_projects.{up,down}.sql` | 1 |
| 40.2 | `rls-users` | S | `internal/migrate/migrations/000102_rls_users.{up,down}.sql` | 1 |
| 40.3 | `rls-organizations` | S | `internal/migrate/migrations/000103_rls_organizations.{up,down}.sql` | 1 |

## Matriz de paralelismo

```
Wave 1 (cero colisión, 3 paralelos):
  40.1  40.2  40.3
```

Cada migración toca archivos disjuntos. Pueden mergearse en cualquier
orden mientras los números secuenciales se mantengan.

## Criterios de éxito globales

- `make migrate-up` aplica 000101, 000102, 000103 sin error.
- Tests de integración existentes (project, user, organization service)
  pasan sin modificación porque todos usan `txctx.WithOrgTx`.
- Test nuevo: `SELECT * FROM projects` desde sesión sin `SET LOCAL` →
  0 rows.
- Test nuevo: `SELECT * FROM projects` con `SET LOCAL app.current_org_id
  = $org_a` → solo proyectos de org_a.
- Test nuevo: idem para `users` y `organizations`.
- `app_admin` (BYPASSRLS) sigue viendo todo (para migrations y batch jobs).
- No hay regresión en latencia (RLS overhead < 5%).
- `make lint-sql` (squawk) pasa sin warnings críticos.

## Prioridad: **alta** (gap de seguridad multi-tenant)

Bloqueante para considerar el sistema "seguro multi-tenant". El riesgo
es bajo porque el código actual es correcto, pero RLS es la red de
seguridad ante futuros bugs y patches descuidados. Idealmente se cierra
junto con REQ-39 (clients también tiene RLS desde 000099).

No requiere despliegue especial: las migrations corren con el
`make migrate-up` estándar y son no-breaking para el código que ya usa
`WithOrgTx`.
