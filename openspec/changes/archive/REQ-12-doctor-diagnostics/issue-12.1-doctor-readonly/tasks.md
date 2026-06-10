# Tasks: issue-12.1-doctor-readonly

## Backend

- [ ] **B1: Crear paquete `internal/doctor/`**
      - `doctor.go` — Doctor(), DoctorReport, CheckResult
      - `checks_project.go` — project health checks
      - `checks_sessions.go` — session integrity checks
      - `checks_sync.go` — sync state checks
      - `checks_db.go` — DB integrity checks

- [ ] **B2: Definir Check type y DoctorReport struct**

- [ ] **B3: Implementar check runner con timeouts**
      - Por cada check, context.WithTimeout(5s)
      - Colectar resultados en categories

- [ ] **B4: Implementar project health checks**
      - checkProjectDirs: existe el directorio del proyecto
      - checkConfigFile: .engram/config.json válido
      - checkGitRemote: git remote reachable

- [ ] **B5: Implementar session integrity checks**
      - checkOpenSessions: sessions sin ended_at > 24h
      - checkOrphanObservations: LEFT JOIN sessions WHERE NULL
      - checkSessionsWithoutObservations: LEFT JOIN observations WHERE NULL

- [ ] **B6: Implementar sync state checks**
      - checkServerReachable: ping al cloud server
      - checkTokenValid: intentar request autenticado
      - checkEnrollmentActive: query enrollment status
      - checkLastSync: cuándo fue la última sync exitosa

- [ ] **B7: Implementar DB integrity checks**
      - checkDBIntegrity: PRAGMA integrity_check
      - checkDiskSpace: os.Stat DB + syscall.Statfs
      - checkMemoryUsage: runtime.ReadMemStats (opcional)

- [ ] **B8: Implementar `engram doctor` CLI**
      - Output table por defecto
      - `--json` flag para output JSON
      - `--category` flag para filtrar

## Tests

- [ ] **T1: Doctor report incluye todas las categorías**
- [ ] **T2: DB integrity check con DB corrupta simula fail**
- [ ] **T3: Orphan observations check detecta orphans**
- [ ] **T4: JSON output es válido**
- [ ] **T5: Doctor no modifica DB (BEGIN + ROLLBACK implícito)**
- [ ] **T6: Timeout en check lento no bloquea otros checks**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/doctor/... -v`
- [ ] Commit: `feat: read-only doctor diagnostics with structured report`
