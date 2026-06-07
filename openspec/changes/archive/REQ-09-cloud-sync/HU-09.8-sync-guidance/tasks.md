# Tasks: HU-09.8-sync-guidance

## Backend

- [ ] **B1: Definir SyncError struct y error codes**
      - `internal/cloud/sync/errors.go`
      - SyncError{Code, Message, Detail}
      - Constantes para códigos conocidos

- [ ] **B2: Implementar IsRepairableCloudSyncError()**
      - `internal/cloud/sync/guidance.go`
      - Mapa de códigos reparables
      - errors.As para type assertion

- [ ] **B3: Implementar GuidanceMessage struct y templates**
      - `internal/cloud/sync/guidance_templates.go`
      - guidanceForCode() switch por código
      - Mensajes: auth_expired, network_timeout, sync_conflict, rate_limited

- [ ] **B4: Implementar BuildGuidance()**
      - `internal/cloud/sync/guidance.go`
      - Formato: título, descripción, pasos numerados, comandos en code blocks
      - fillDetail() para incluir Detail del error

- [ ] **B5: Integrar guidance en sync error handling**
      - Llamar guidance cuando ocurre error de sync
      - Log + print guidance

## Tests

- [ ] **T1: IsRepairableCloudSyncError(auth_expired) → true**
- [ ] **T2: IsRepairableCloudSyncError(network_timeout) → true**
- [ ] **T3: IsRepairableCloudSyncError(sync_conflict) → true**
- [ ] **T4: IsRepairableCloudSyncError(rate_limited) → true**
- [ ] **T5: IsRepairableCloudSyncError(internal_error) → false**
- [ ] **T6: IsRepairableCloudSyncError(unknown) → false**
- [ ] **T7: IsRepairableCloudSyncError(error_sin_code) → false**
- [ ] **T8: BuildGuidance(auth_expired) incluye "Re-authentication" y "engram cloud auth login"**
- [ ] **T9: BuildGuidance(network_timeout) incluye "check your internet"**
- [ ] **T10: BuildGuidance(sync_conflict) incluye "engram doctor" y "engram repair"**
- [ ] **T11: BuildGuidance(internal_error) retorna ""**
- [ ] **T12: Guidance incluye pasos numerados y comandos formateados**
- [ ] **T13: Sabotaje — guidanceForCode("") no retorna nil → panic → test cae → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/cloud/sync/... -v`
- [ ] Commit: `feat: sync guidance system for repairable errors`
