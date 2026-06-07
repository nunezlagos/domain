# Proposal: HU-12.3-health-version

## Intención

Proveer un endpoint /health para monitoreo de servicio y un comando `engram version` para información de versión. Soporte para systemd y launchd healthchecks.

## Scope

**Incluye:**
- GET /health endpoint (200 OK, 503 degraded)
- `engram version` CLI command (texto y --json)
- Version package con ldflags injection
- Uptime tracking (start time)
- DB connectivity check en /health
- Sin autenticación en /health

**No incluye:**
- Métricas detalladas (Prometheus) — futuro
- Health check customizables (usa doctor checks implícitamente)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Version | Package `internal/version` con variables seteadas por ldflags |
| Uptime | `time.Since(startTime)` donde startTime se setea en init |
| Health handler | Simple: ping DB + version + uptime |
| DB check | `db.PingContext(ctx, 2s timeout)` |
| systemd | Exit 0 → healthy; exit 1 → degraded; compatible con `HealthCheckCommand` |
| launchd | Responde 200/503; launchd puede parsear JSON |

