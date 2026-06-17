# Design: issue-21.5-single-org-collapse

## Decisión arquitectónica

**Single-org se logra removiendo la *gestión de N orgs*, NO destruyendo el plumbing de 1 org.**

El modelo de datos multi-tenant (columna `organization_id` + RLS por `current_org_id()`)
funciona correctamente cuando existe exactamente una org: cada request resuelve la misma
org desde la API key, el GUC `app.current_org_id` se setea a ese único UUID, y RLS aísla
contra esa org (que es todo el dataset). Por eso el plumbing se preserva como **defense in
depth gratis** y se difiere su remoción destructiva a issue-21.6.

Lo que se elimina es el surface que solo tiene sentido con N>1 orgs:
- lifecycle (crear/borrar/transferir orgs)
- invitaciones email cross-org (reemplazadas por enrollment-tokens single-org)

## Matriz de decisión por componente

| Componente | Acción | Razón |
|------------|--------|-------|
| `POST /organizations` (createOrg) | BORRAR | nunca se crea una 2ª org |
| `DELETE /organizations/{id}` (deleteOrg) | BORRAR | nunca se borra la única org por API |
| `POST .../transfer-ownership` | BORRAR | no hay a quién transferir entre orgs |
| `GET /organizations/{id}` (getOrg) | PRESERVAR | leer settings de la única org |
| `PATCH /organizations/{id}` (updateOrg) | PRESERVAR | editar settings de la única org |
| `GET/POST .../members` | PRESERVAR | gestión de usuarios de la org |
| `.../enrollment-token/*`, `/auth/enroll` | PRESERVAR | onboarding single-org (issue-37) |
| `/admin/org-overview` | PRESERVAR | dependencia REQ-41 (deployado, healthy) |
| `service/org` Create/SoftDelete/Transfer/AddMember(legacy) | BORRAR | lifecycle multi-org |
| `service/org` GetByID/UpdateSettings/ListMembers/AddMemberWithAPIKey | PRESERVAR | la única org |
| `service/org/delete.go` (DeleteService) | BORRAR | hard-delete de orgs |
| `cmd/domain/org_delete.go` + dispatch | BORRAR | CLI de borrado de orgs |
| `service/invite` + `handler/invite.go` + rutas | BORRAR | reemplazado por enrollment-tokens |
| bootstrap (`install_cli.go`) crea org Local | PRESERVAR | crea LA única org |
| SDK Organizations create/update/delete | BORRAR | gestión multi-org |
| columnas `organization_id`, RLS, GUC, resolución API key | PRESERVAR | plumbing 1-org (→21.6) |

## Alternativas descartadas

- **Renombrar `/organizations/{id}` → `/settings` y `/members`:** churn cosmético que rompe
  SDK y clientes existentes sin beneficio funcional. Se difiere; las rutas actuales sirven.
- **Drop destructivo del schema en esta misma HU:** alto riesgo sobre datos de producción
  (54 tablas, 658 refs Go), irreversible. Se separa en issue-21.6 con plan incremental.
- **Hardcodear un `DEFAULT_ORG_ID` constante:** innecesario — la org se resuelve desde la
  API key igual que hoy; con una sola org el resultado es siempre el mismo UUID.

## Plan de verificación

1. `go build ./...` — falla si quedan refs colgadas a símbolos borrados (createOrg, invite, etc.).
2. `go vet ./...`.
3. Ajustar/borrar tests de lifecycle (`service/org/*_test.go` de Create/Delete/Transfer, invite tests).
4. Smoke VPS: rutas preservadas → 200; rutas removidas → 404.

## Nota de continuidad

issue-21.6 (org-schema-decommission) consumirá este estado: con el surface ya colapsado,
el drop de columnas/RLS/tabla queda como una migración de datos pura, sin código de gestión
multi-org que la bloquee.
