# Proposal: HU-09.5-cloud-autosync

## Intención

Implementar un manager de sincronización background que ejecuta ciclos periódicos de push/pull con una state machine robusta, backoff exponencial en errores, y reason codes para diagnóstico.

## Scope

**Incluye:**
- AutosyncManager con goroutine background
- State machine: idle → pushing → pulling → healthy (o failed → backoff → pushing)
- Reason codes: network_error, auth_error, server_error, timeout, rate_limited
- Backoff exponencial: 30s, 60s, 120s, 240s, max 5min
- Intervalo configurable (default 60s)
- GET /api/cloud/sync-status endpoint
- ENGRAM_CLOUD_AUTOSYNC env var control

**No incluye:**
- Conflict resolution (REQ-10)
- Audit logging (HU-09.6)
- Dashboard UI integration (HU-09.4)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Manager | Goroutine con ticker y select; context cancel para stop |
| State machine | Enum con transiciones explícitas + channel de eventos |
| Backoff | Slice de duraciones; reset en healthy; tope 5min |
| Reason codes | Enum con strings; se limpia en healthy |
| Thread safety | sync.RWMutex para acceso concurrente al estado |
| Status API | Handler que lee estado con RLock y retorna JSON |

