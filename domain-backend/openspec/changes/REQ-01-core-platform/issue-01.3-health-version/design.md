# Design: issue-01.3-health-version

## Decisión arquitectónica

**Router:** `net/http` estándar con `http.ServeMux` (Go 1.22+) para evitar dependencias externas.
**Version package:** Paquete dedicado `internal/version` con variables que ldflags sobreescribe.
**CLI:** Subcomando en `cmd/domain/version.go` usando `flag` de stdlib.

## Alternativas descartadas

- **chi/gin solo para health:** Sobredimensionado. `net/http` es suficiente para un endpoint sencillo.
- **Cobra para CLI:** Demasiado para un solo subcomando. `flag` de stdlib alcanza.
- **Health check con DB query:** PingContext es más liviano que ejecutar SELECT 1.

## Diagrama

```
GET /health
  │
  ├─→ Ping DB (context timeout 3s)
  │     ├─→ ok  → db_alive: true
  │     └─→ err → db_alive: false
  │
  ├─→ status = (db_alive && started) ? "ok" : "degraded"
  ├─→ httpStatus = (status == "ok") ? 200 : 503
  │
  └─→ Response JSON:
        {
          "status": "ok" | "degraded",
          "version": "1.0.0" | "dev",
          "commit": "abc123",
          "build_time": "2026-06-07T10:00:00Z",
          "uptime": "1m2.5s",
          "db_alive": true | false
        }
```

## Version package

```go
package version

var (
    Version   = "dev"
    Commit    = "none"
    BuildTime = "unknown"
)
```

## Makefile ldflags

```makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := -X domain/internal/version.Version=$(VERSION)
LDFLAGS += -X domain/internal/version.Commit=$(COMMIT)
LDFLAGS += -X domain/internal/version.BuildTime=$(DATE)
```

## TDD plan

1. Test handler 200 con DB mock funcional
2. Test handler 503 con DB mock que retorna error
3. Test uptime es positivo
4. Test version default es "dev" sin ldflags
5. Test version se sobreescribe con ldflags simuladas
6. Test CLI `domain version` imprime version/commit/buildtime
7. Test response JSON tiene todos los campos esperados

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| DB ping timeout lento | Baja | Medio | Context timeout 3s |
| ldflags olvidadas en build | Media | Bajo | Default "dev" visible |
| Uptime overflow | Muy baja | Bajo | time.Duration maneja hasta 290 años |
