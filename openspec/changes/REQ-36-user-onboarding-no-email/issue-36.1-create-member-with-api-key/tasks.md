# Tasks: issue-36.1-create-member-with-api-key

## Backend

- [ ] **T1**: Extraer `bootstrap.generateAPIKey` a paquete compartido:
  - Nuevo: `internal/auth/apikey/keygen.go`
  - Export: `GenerateLiveKey() (plaintext string, hash []byte, prefix string, err error)`
  - Mover el constants + algoritmo desde `bootstrap/service.go`
  - Update `bootstrap/service.go` para usar el nuevo helper

- [ ] **T2**: Errores tipados en `orgsvc`:
  - `ErrInvalidEmail`
  - `ErrInvalidRole`
  - `ErrEmailTaken` (mapeado desde unique violation users_org_email_uniq)
  - `ErrOrgNotFound` (ya existe)

- [ ] **T3**: `orgsvc.Service.AddMemberWithAPIKey`:
  - Signature: `(ctx, orgID, actorID, email, name, role) → (*MemberWithKey, error)`
  - Validaciones (email regex igual al bootstrap, role whitelist)
  - Begin tx
  - Check org existe + not deleted
  - INSERT users con dummy bcrypt password_hash
  - Generate key via `apikey.GenerateLiveKey()`
  - INSERT api_keys (env=live, expires NULL, name="default")
  - Audit recorder.Record("member.created_with_key", ...)
  - Commit
  - Return plaintext + IDs

- [ ] **T4**: Handler `addMemberWithKey` en `internal/api/handler/org.go`:
  - Parse orgID del path
  - RBAC: principal owner/admin de la org (no de otra)
  - Decode body { email, name, role }
  - Llamar service
  - 201 + Location + writeData con shape del Escenario 1
  - Mapear errores

- [ ] **T5**: Wire en `internal/api/handler/api.go`:
  - `mux.HandleFunc("POST /api/v1/organizations/{id}/members", a.addMemberWithKey)`
  - Cerca de las otras rutas de organizations

- [ ] **T6**: response-shape-lint snapshot regen:
  - `go test -run TestRegenSnapshots_Manual ./cmd/response-shape-lint -args -regen`
  - Verificar que `endpoint_shapes.json` y `error_codes.json` se actualizan

## Tests

- [ ] **T-unit-1**: `apikey.GenerateLiveKey` formato:
  - plaintext empieza con "domk_live_"
  - plaintext tiene 42 chars (10 prefix + 32 random)
  - hash es bcrypt válido (Cost=12)
  - prefix tiene 18 chars ("domk_live_xxxxxxxx")

- [ ] **T-unit-2**: validaciones de `AddMemberWithAPIKey` sin DB:
  - Email inválido → `ErrInvalidEmail`
  - Role inválido → `ErrInvalidRole`
  - Role valido pero name vacío → OK

- [ ] **T-integration-1** (testcontainers):
  - Setup org+owner
  - Llamar `AddMemberWithAPIKey(ctx, orgID, ownerID, "alice@x.com", "Alice", "member")`
  - Verificar user creado, api_key creada, hash matches plaintext
  - Verificar audit_log entry con action="member.created_with_key"
  - Verificar 2do call mismo email → ErrEmailTaken + ningún user/key extra

- [ ] **T-integration-2** (testcontainers):
  - Handler test con auth middleware mockeado
  - POST con principal owner → 201 + body con api_key
  - POST con principal member → 403
  - POST con email duplicado → 409
  - POST con email inválido → 422
  - POST con role inválido → 422

- [ ] **T-sabotaje**: comentar el INSERT api_keys (mantener solo INSERT users)
  - Test e2e que assserta "plaintext debe estar en response" DEBE FALLAR
  - Restaurar el INSERT → test verde

## Documentación

- [ ] **T7**: README o docs/onboarding.md (si existe): documentar el nuevo endpoint
- [ ] **T8**: state.yaml → implemented
