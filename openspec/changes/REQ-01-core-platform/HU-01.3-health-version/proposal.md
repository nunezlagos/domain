# Proposal: HU-01.3-health-version

## Intención

Exponer un endpoint `GET /health` que permita a operadores y sistemas de monitoreo verificar el estado del servicio. La versión debe ser inyectada en build time y accesible tanto por HTTP como por CLI.

## Scope

**Incluye:**
- Endpoint `GET /health` con response JSON
- Health check de base de datos (ping pool)
- Status "ok" (200) vs "degraded" (503)
- Uptime desde inicio del proceso
- Package `internal/version` con variables `Version`, `Commit`, `BuildTime`
- ldflags en Makefile para inyectar version/commit/buildtime
- Comando CLI `domain version`

**No incluye:**
- Health checks de dependencias externas (LLM APIs, Redis, etc.)
- Readiness probe vs liveness probe separados
- Métricas de performance (prometheus, etc.)

## Enfoque técnico

1. Router HTTP (net/http o chi), handler en `/health`
2. DB ping con `(*sql.DB).PingContext()` con timeout de 3s
3. Uptime calculado como `time.Since(startTime)` guardado al inicio de `main()`
4. `internal/version/version.go` con variables exportadas
5. `Makefile` con ldflags: `-X domain/internal/version.Version=$(VERSION) -X domain/internal/version.Commit=$(COMMIT)`
6. CLI subcommand con `urfave/cli` v2 o manual con `flag`

## Riesgos

- **Timeout en DB ping:** Si la DB está lenta, puede bloquear el health check. Mitigación: Context con timeout.
- **Version no inyectada:** Mostrar "dev" como fallback para builds sin ldflags.
- **Uptime reseteado en cada deploy:** Comportamiento esperado; no es un problema.

## Testing

- Test handler devuelve 200 con DB mock funcional
- Test handler devuelve 503 con DB mock que falla
- Test version package con ldflags simuladas
- Test CLI command
- Test de integración contra Postgres real (opcional)
