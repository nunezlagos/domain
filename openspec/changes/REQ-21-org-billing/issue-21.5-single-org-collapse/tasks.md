# Tasks: issue-21.5-single-org-collapse

> **Nota de ejecución (2026-06-17):** se removió la EXPOSICIÓN externa de gestión
> multi-org (rutas API + endpoint admin + CLI). Los métodos del service layer
> (`Create`/`TransferOwnership`/`SoftDelete`) se RETIENEN como helpers internos:
> los usan 23 integration tests como fixture de creación de org y el bootstrap
> conceptualmente. Su remoción definitiva se difiere a issue-21.6 (junto con el
> drop del schema), evitando un churn de 23 archivos de test sin beneficio funcional.

## Rutas y handlers
- [x] **sc-001**: Quitar rutas lifecycle en `api.go` (POST /organizations, DELETE /organizations/{id}, transfer-ownership)
- [x] **sc-002**: Borrar handlers `createOrg`, `deleteOrg`, `transferOwnership` en `handler/org.go`; conservar get/update/members
- [x] **sc-002b**: Borrar endpoint admin de borrado de org `handler/admin/delete_handler.go` (era código muerto, sin ruta montada)

## Service org
- [x] **sc-003**: Borrar `internal/service/org/delete.go` (DeleteService) — solo lo usaban el endpoint admin y el CLI, ambos removidos
- [~] **sc-004**: Métodos `Create`/`SoftDelete`/`TransferOwnership` RETENIDOS como helpers internos (ver nota). Remoción → issue-21.6

## CLI
- [x] **sc-005**: Borrar `cmd/domain/org_delete.go` (`runOrgCmd`/`runOrgDelete` — estaban huérfanos, sin dispatch en main.go)

## Invitations (reemplazado por enrollment-tokens)
- [ ] **sc-006**: Borrar `internal/service/invite/` + `handler/invite.go` + rutas `/organizations/{id}/invitations` — PENDIENTE (subsistema separado, se trata aparte)

## SDK
- [ ] **sc-007**: Podar Organizations create/update/delete en `sdks/go`, `sdks/python`, `sdks/typescript` — PENDIENTE

## Verificación
- [x] **sc-009**: `go build ./...` Success + `go vet ./...` (default) sin issues. Integration: solo 3 fallos PRE-EXISTENTES (txctx/knowledge/e2e, no-org, confirmados en baseline)
- [ ] **sc-010**: Smoke VPS: rutas preservadas 200, removidas 404, admin dashboard healthy — PENDIENTE (post-deploy)
- [ ] **deploy-001**: Deploy al VPS — PENDIENTE (requiere confirmar mecanismo: host `vps` + bump `DOMAIN_BACKEND_VERSION` + `make pull` / registry)
