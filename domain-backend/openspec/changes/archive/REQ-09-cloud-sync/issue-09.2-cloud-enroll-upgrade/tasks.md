# Tasks: issue-09.2-cloud-enroll-upgrade

## Backend

- [ ] **B1: Implementar CloudState enum y validaciones**
      - `internal/cloud/state.go`
      - Estados: none, configured, enrolled, upgraded, error
      - `CanTransitionTo(target) bool`

- [ ] **B2: Implementar Enroll() function**
      - `internal/cloud/enroll.go`
      - POST /api/enroll con machine_id, hostname, version
      - Parse response: enrollment_id, server_version
      - Actualizar cloud.json con enrollment_id, state, enrolled_at
      - Backup antes de escribir

- [ ] **B3: Implementar getMachineID()**
      - Leer /etc/machine-id
      - Fallback: hostid + hostname hash
      - Último recurso: UUID generado y persistido en cloud.json

- [ ] **B4: Implementar DoctorCheck y doctorChecks**
      - `internal/cloud/doctor.go`
      - 4 checks: config_exists, server_reachable, token_valid, enrollment_active

- [ ] **B5: Implementar upgrade subcomandos CLI**
      - `engram upgrade doctor` — ejecuta checks, output table/json
      - `engram upgrade repair` — doctor + auto-fix actions
      - `engram upgrade bootstrap` — wizard interactivo
      - `engram upgrade rollback` — restore cloud.json.bak
      - `engram upgrade status` — mustra estado actual

- [ ] **B6: Implementar backup/rollback helpers**
      - `backupConfig(path)` → crea cloud.json.bak
      - `rollbackConfig(path)` → restaura desde .bak

- [ ] **B7: Integrar `engram cloud enroll` CLI**
      - Validar estado actual
      - Llamar Enroll()
      - Mostrar resultado

## Tests

- [ ] **T1: State transitions válidas e inválidas**
- [ ] **T2: Enroll exitoso (mock server)**
- [ ] **T3: Enroll falla si ya enrolled**
- [ ] **T4: Doctor checks retorna 4 resultados**
- [ ] **T5: Rollback requiere backup existente**
- [ ] **T6: Rollback restaura config correctamente**
- [ ] **T7: Sabotaje — permitir transición inválida → test cae → restaurar**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/cloud/... -v`
- [ ] Commit: `feat: cloud enrollment and upgrade lifecycle with state machine`
