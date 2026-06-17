# Proposal: issue-21.5-single-org-collapse

## Intención

Colapsar el surface de gestión multi-organización a un modelo single-org, eliminando
las operaciones de ciclo de vida de múltiples orgs (crear, borrar, transferir) y los
sistemas paralelos de onboarding cross-org (invitaciones email), preservando intacto
el plumbing single-tenant (org_id + RLS) y los features que dependen de la única org.

## Scope

**Incluye (BORRAR):**
- Rutas HTTP: `POST /organizations`, `DELETE /organizations/{id}`, `POST /organizations/{id}/transfer-ownership`
- Handlers: `createOrg`, `deleteOrg`, `transferOwnership` (en `internal/api/handler/org.go`)
- Service: `internal/service/org/delete.go` (DeleteService completo), métodos
  `Create`, `SoftDelete`, `TransferOwnership` y `AddMember` (el legacy sin API key) de `service.go`
- CLI: `cmd/domain/org_delete.go` + dispatch `runOrgCmd`/`runOrgDelete` en `main.go`
- Invitations (sistema paralelo, reemplazado por enrollment-tokens):
  - Service `internal/service/invite/`, handlers `internal/api/handler/invite.go`,
    rutas `/organizations/{id}/invitations`
- SDK org management (los 3 lenguajes): métodos `Create`/`Update`/`Delete` del recurso
  Organizations en `sdks/go`, `sdks/python`, `sdks/typescript`

**Preserva (NO tocar):**
- Bootstrap/install crea la única org (`Local`/`local`) — sin cambios
- `GET`/`PATCH /organizations/{id}` (leer/actualizar settings de la única org)
- `GET`/`POST /organizations/{id}/members` (gestión de usuarios en la org)
- enrollment-token (issue-37) + enroll self + add-member-with-key (issue-36)
- `GET /api/v1/admin/org-overview` (dependencia de REQ-41 admin dashboard)
- Plumbing single-tenant: columnas `organization_id`, RLS `current_org_id()`,
  `SET LOCAL app.current_org_id` en middleware, resolución de org desde API key

**No incluye (→ issue-21.6):**
- Drop de columnas `organization_id` en las 54 tablas
- Drop de RLS policies org y función `current_org_id()`
- Drop de la tabla `organizations` y tablas satélite (usage_counters, plans, etc.)

## Enfoque técnico

1. Quitar rutas de lifecycle en `api.go` (create/delete/transfer).
2. Borrar handlers correspondientes; mantener get/update/members.
3. Podar `service/org`: borrar `delete.go`, remover métodos lifecycle de `service.go`.
4. Borrar CLI `org delete` + su dispatch.
5. Remover invitations end-to-end (service + handler + rutas).
6. Podar SDKs (create/update/delete org).
7. `go build ./...` verde + `go vet`.
8. Deploy al VPS.

## Riesgos

- **Romper REQ-41 admin dashboard:** mitigado — `org-overview` se preserva explícitamente.
- **Romper onboarding:** mitigado — enrollment-tokens + add-member se preservan.
- **Referencias colgadas tras borrar invitations/org methods:** mitigado por `go build ./...`
  que falla si queda algo sin resolver; se corrigen iterando hasta verde.

## Testing

- `go build ./...` y `go vet ./...` verdes.
- Tests de `service/org` ajustados (borrar los de lifecycle removido).
- Smoke en VPS: GET /organizations/{id}, members, org-overview responden 200; rutas
  removidas dan 404.
