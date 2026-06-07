# Tasks: HU-12.3-health-version

## Backend

- [ ] **B1: Crear `internal/version/` package**
      - `version.go` — variables Version, Commit, BuildDate
      - `info.go` — Info struct, GetInfo()

- [ ] **B2: Configurar ldflags en build**
      - Makefile o task runner con VERSION, COMMIT, DATE
      - -ldflags con las variables del version package

- [ ] **B3: Implementar healthHandler**
      - `internal/api/health.go`
      - db.PingContext con timeout 2s
      - uptime tracking (startTime package var)
      - 200 si ok, 503 si degraded

- [ ] **B4: Implementar `engram version` CLI**
      - Output: "memoria vX.Y.Z (commit) date\ngoX.Y OS/arch"
      - --json flag para JSON

- [ ] **B5: Crear archivo de servicio systemd (docs)**
      - `contrib/memoria.service`
      - HealthCheck via /health endpoint

- [ ] **B6: Crear plist para launchd (docs)**
      - `contrib/com.memoria.store.plist`

## Tests

- [ ] **T1: /health retorna 200 con DB**
- [ ] **T2: /health retorna 503 sin DB**
- [ ] **T3: /health incluye uptime y timestamp**
- [ ] **T4: /health responde en < 100ms**
- [ ] **T5: /health no requiere auth**
- [ ] **T6: `engram version` output correcto**
- [ ] **T7: `engram version --json` output JSON válido**

## Cierre

- [ ] `go build ./...` sin errores
- [ ] `go vet ./...` sin warnings
- [ ] `go test ./internal/api/... -v`
- [ ] `go test ./internal/version/... -v`
- [ ] Commit: `feat: health endpoint and version command`
