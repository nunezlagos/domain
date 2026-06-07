# Design: HU-12.3-health-version

## Decisión arquitectónica

### Version package

```go
// internal/version/version.go
package version

var (
    Version   = "dev"
    Commit    = "none"
    BuildDate = "unknown"
    GoVersion  = runtime.Version()
    OS         = runtime.GOOS
    Arch       = runtime.GOARCH
)

type Info struct {
    Version   string `json:"version"`
    Commit    string `json:"commit"`
    BuildDate string `json:"build_date"`
    GoVersion string `json:"go_version"`
    OS        string `json:"os"`
    Arch      string `json:"arch"`
}

func GetInfo() Info {
    return Info{
        Version: Version, Commit: Commit, BuildDate: BuildDate,
        GoVersion: GoVersion, OS: OS, Arch: Arch,
    }
}
```

### Build ldflags

```makefile
VERSION := $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -ldflags="\
    -X 'github.com/nunezlagos/memoria/internal/version.Version=$(VERSION)' \
    -X 'github.com/nunezlagos/memoria/internal/version.Commit=$(COMMIT)' \
    -X 'github.com/nunezlagos/memoria/internal/version.BuildDate=$(DATE)'"
```

### Health handler

```go
// internal/api/health.go
var startTime = time.Now()

type HealthResponse struct {
    Status    string `json:"status"`    // ok, degraded
    Version   string `json:"version"`
    Uptime    string `json:"uptime"`
    DB        string `json:"db"`        // connected, disconnected
    Timestamp string `json:"timestamp"`
}

func healthHandler(db *sql.DB) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
        defer cancel()

        resp := HealthResponse{
            Version:   version.Version,
            Uptime:    time.Since(startTime).Round(time.Second).String(),
            Timestamp: time.Now().UTC().Format(time.RFC3339),
        }

        if err := db.PingContext(ctx); err != nil {
            resp.Status = "degraded"
            resp.DB = "disconnected"
            w.WriteHeader(http.StatusServiceUnavailable)
        } else {
            resp.Status = "ok"
            resp.DB = "connected"
        }

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(resp)
    }
}
```

### Version CLI

```
$ engram version
memoria v1.2.3 (abc1234) 2026-06-07T00:00:00Z
go1.22 linux/amd64

$ engram version --json
{"version":"v1.2.3","commit":"abc1234","build_date":"2026-06-07T00:00:00Z",...}
```

### systemd service unit

```ini
[Unit]
Description=Domain Memory Store
After=network.target

[Service]
ExecStart=/usr/local/bin/memoria serve
ExecHealthCheck=/usr/local/bin/memoria doctor --json
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

O usando el endpoint HTTP:
```ini
HealthCheckCommand=curl -f http://localhost:8080/health
```

## Alternativas descartadas

| Alternativa | Razón de descarte |
|-------------|-------------------|
| /health con auth | Los healthchecks de systemd/launchd no tienen auth; debe ser público |
| Métricas detalladas en /health | Debe ser rápido (< 100ms); métricas en otro endpoint |
| Version en archivo separado | ldflags injection es standard en Go; más fácil que mantener archivo |

## TDD plan

1. **Red:** /health retorna 200 con DB conectada → falla
2. **Green:** Implement healthHandler → pasa
3. **Red:** /health retorna 503 con DB desconectada → falla
4. **Green:** Implement db.PingContext check → pasa
5. **Red:** `engram version` output formateado → falla
6. **Green:** Implement version CLI → pasa

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| ldflags no seteadas en build de desarrollo | Default "dev" es aceptable; warning visible |
| DB Ping timeout muy largo | 2s timeout; si DB no responde rápido, degraded |
| Uptime reset en restart | Comportamiento esperado; startTime se setea en init |
