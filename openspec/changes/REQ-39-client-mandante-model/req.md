# REQ-39 — Client / Mandante Model (clientes como entidad de primera clase)

> **Origen**: sesión 2026-06-14. La organización operadora (`nunezlagos`,
> consultora) gestiona proyectos que pertenecen a clientes/mandantes externos
> (`acme-corp`, `globex`, etc.). Hoy `projects` cuelga directo de `organizations`
> y no hay forma de modelar la contraparte. Se introduce `clients` como entidad
> per-org y se extiende `projects.client_id` (opcional, NULL = proyecto interno
> de la org). El mismo schema cubre el caso single-org-sin-clientes y el caso
> multi-cliente, sin SaaS, sin facturación, sin invitaciones cross-tenant.

## Contexto

Estado actual del modelo multi-tenant (post REQ-01..38):

```
organizations (root tenant)
  ├── users           (FK organization_id)
  └── projects        (FK organization_id, slug UNIQUE per-org)
        └── observations / sessions / agents / ...
```

La organización es a la vez "el tenant" y "el proveedor del servicio". No hay
forma de decir "este proyecto es para el cliente Acme". La consultora trabaja
con N mandantes y necesita:

1. Listar clientes (`acme-corp`, `globex`, `initech`).
2. Crear proyectos asociados a un cliente (`/clients/acme-corp/projects`).
3. Conservar proyectos internos (sin cliente, ej. ops internas).
4. Aislar datos de clientes entre orgs (Acme de la consultora A no debe ser
   visible para consultora B).

Objetivo del REQ:

```
organizations
  ├── users
  ├── clients         (NUEVO — cuentas/mandantes per-org)
  │     └── projects  (FK client_id NULLABLE)
  └── projects        (client_id = NULL → proyecto interno de la org)
```

## Restricciones de diseño

1. **No SaaS**: 20 users en total escala. Sin invitaciones cross-tenant, sin
   billing per-client, sin RBAC granular client-scope. Solo aislamiento de
   datos y organización lógica.
2. **`client_id` opcional**: NULL = proyecto interno de la organización
   operadora. NOT NULL = proyecto para un mandante concreto. No se fuerza
   migración de proyectos viejos.
3. **Same-org enforcement a nivel DB**: `projects.organization_id` debe ser
   igual a `clients.organization_id` cuando `client_id IS NOT NULL`. Se
   implementa con trigger BEFORE INSERT/UPDATE (Postgres no permite subqueries
   en `CHECK`).
4. **Slug único per-org**: `UNIQUE (organization_id, slug)` en `clients`.
   Cliente `acme-corp` puede existir en dos orgs distintas sin colisión.
5. **Soft delete**: columna `deleted_at TIMESTAMPTZ` con índice parcial
   `WHERE deleted_at IS NULL`. Coherente con el patrón ya usado en
   organizations/users/projects.
6. **Status acotado**: `status VARCHAR(20)` con `CHECK (status IN
   ('active','inactive','archived'))`. Sin máquina de estados; transición libre.
7. **`projects.client_id` con `ON DELETE SET NULL`**: si se borra (hard) un
   cliente, los proyectos sobreviven huérfanos (NULL) en vez de cascadear y
   perder histórico. Cascadear sería destructivo para una consultora.
8. **Sin cambios al modelo de users**: el operador es un user de la org
   operadora. No se modela `client_users` (los clientes no se loggean al
   sistema en este REQ — eso sería otro requerimiento).
9. **MCP y REST simétricos**: cada operación CRUD debe estar disponible vía
   HTTP REST y vía MCP tool para mantener consistencia con REQ-12 y REQ-31.
10. **Trabajable en paralelo**: cada HU toca archivos disjuntos para permitir
    branches simultáneas sin colisión.

## HUs

| HU | Slug | Esfuerzo | Archivos tocados | Wave |
|----|------|----------|------------------|------|
| 39.1 | `clients-schema` | S | `internal/migrate/migrations/000099_create_clients.{up,down}.sql` | 1 |
| 39.2 | `projects-client-id-extension` | S | `internal/migrate/migrations/000100_projects_add_client_id.{up,down}.sql` | 1 |
| 39.3 | `clients-service-and-repo` | M | `internal/service/client/{service,repository,pg_repository}.go` | 2 |
| 39.4 | `clients-rest-handlers` | M | `internal/api/handler/client.go` (+ wiring router) | 3 |
| 39.5 | `clients-mcp-tools` | M | `internal/mcp/server/client_tools.go` | 3 |
| 39.6 | `projects-accept-client-slug` | M | `internal/service/project/*.go`, `internal/api/handler/project.go`, `internal/mcp/server/project_tools.go` | 4 |

## Matriz de paralelismo

```
Wave 1 (cero colisión, 2 paralelos):
  39.1  39.2

Wave 2 (cuando Wave 1 corrió migrations):
  39.3  ← depende 39.1

Wave 3 (cuando 39.3 expone el service):
  39.4  ← depende 39.3
  39.5  ← depende 39.3

Wave 4:
  39.6  ← depende 39.2, 39.3 (necesita client_id en projects y resolver client por slug)
```

Cada wave es mergeable a `services` sin conflictos porque los archivos tocados
son disjuntos dentro de la wave.

## Criterios de éxito globales

- `make migrate-up` aplica migraciones 099 y 100 sin error.
- `POST /api/v1/clients` con `{name, slug}` crea un cliente bajo la org del
  caller y devuelve 201 con id + slug.
- `GET /api/v1/clients` lista solo clientes de la org del caller (multi-tenant
  isolation verificado en test de integración con 2 orgs).
- `POST /api/v1/projects` aceptando `client_slug` resuelve a `client_id` antes
  de insertar; si el slug no existe en la org del caller devuelve 422.
- `POST /api/v1/projects` sin `client_slug` crea proyecto interno
  (`client_id = NULL`) — comportamiento legacy preservado.
- `INSERT INTO projects (organization_id, client_id, slug) VALUES ($org_a,
  $client_de_org_b, 'x')` falla con `check_violation` (trigger same-org).
- `DELETE FROM clients WHERE id=$x` (hard delete) deja `projects.client_id =
  NULL` para los proyectos asociados, sin cascade-delete.
- Tools MCP `client.create`, `client.list`, `client.get`, `client.update`,
  `client.archive` funcionan vía `POST /mcp` con el mismo aislamiento por org.
- Test cross-tenant: org A no ve clientes de org B en ninguna ruta REST ni
  MCP, ni vía SQL si la app olvidara WHERE (cubierto por RLS — ver REQ-40).

## Prioridad: **media-alta**

Bloqueante para que el dashboard (REQ-32) pueda mostrar "Proyectos por
cliente" y para que el flujo de la consultora refleje el modelo real de
negocio. No bloquea deploy del VPS (REQ-38) ni MCP (REQ-31).
