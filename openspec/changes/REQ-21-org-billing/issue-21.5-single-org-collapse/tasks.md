# Tasks: issue-21.5-single-org-collapse

## Rutas y handlers
- [ ] **sc-001**: Quitar rutas lifecycle en `api.go` (POST /organizations, DELETE /organizations/{id}, transfer-ownership)
- [ ] **sc-002**: Borrar handlers `createOrg`, `deleteOrg`, `transferOwnership` en `handler/org.go`; conservar get/update/members

## Service org
- [ ] **sc-003**: Borrar `internal/service/org/delete.go` (DeleteService)
- [ ] **sc-004**: Remover métodos `Create`, `SoftDelete`, `TransferOwnership`, `AddMember` (legacy) de `service/org/service.go`; conservar `GetByID`, `UpdateSettings`, `ListMembers`, `AddMemberWithAPIKey`

## CLI
- [ ] **sc-005**: Borrar `cmd/domain/org_delete.go` + dispatch `runOrgCmd`/`runOrgDelete` en `main.go`

## Invitations (reemplazado por enrollment-tokens)
- [ ] **sc-006**: Borrar `internal/service/invite/` + `handler/invite.go` + rutas `/organizations/{id}/invitations`

## SDK
- [ ] **sc-007**: Podar Organizations create/update/delete en `sdks/go`, `sdks/python`, `sdks/typescript` (conservar get/list members si aplica)

## Tests
- [ ] **sc-008**: Borrar/ajustar tests de lifecycle removido (`service/org` Create/Delete/Transfer, invite tests)

## Verificación
- [ ] **sc-009**: `go build ./...` + `go vet ./...` verdes
- [ ] **sc-010**: Smoke VPS: rutas preservadas 200, removidas 404, admin dashboard healthy
- [ ] **deploy-001**: Deploy al VPS (req-41.4m)
