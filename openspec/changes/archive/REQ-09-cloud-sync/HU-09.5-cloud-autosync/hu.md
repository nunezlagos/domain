# HU-09.5-cloud-autosync

**Origen:** `REQ-09-cloud-sync`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria cloud
**Quiero** que la sincronización ocurra automáticamente en background
**Para** no tener que ejecutar push/pull manualmente cada vez

**Como** usuario
**Quiero** controlar el autosync con ENGRAM_CLOUD_AUTOSYNC=1
**Para** habilitarlo o deshabilitarlo según mi necesidad

## Criterios de aceptación

```gherkin
Scenario: Autosync inicia en background al habilitarlo
  Given ENGRAM_CLOUD_AUTOSYNC=1
  When se inicia memoria
  Then un manager background comienza a ejecutar ciclos de sync

Scenario: State machine transiciona entre fases
  Given el autosync manager está corriendo
  When comienza un ciclo de sync
  Then la fase cambia a "pushing"
  When el push completa exitosamente
  Then la fase cambia a "pulling"
  When el pull completa exitosamente
  Then la fase cambia a "healthy"

Scenario: Push falla y transiciona a failed
  Given el servidor cloud no responde
  When se ejecuta un push
  Then la fase cambia a "failed"
  And se registra un reason_code

Scenario: Failed transiciona a backoff con espera exponencial
  Given la fase es "failed" después de un error
  When pasan los segundos de backoff
  Then la fase cambia a "backoff"
  And el próximo reintento usa delay: 30s, 60s, 120s, 240s (max 5min)

Scenario: Backoff exitoso transiciona a pushing again
  Given la fase es "backoff" y el tiempo de espera expiró
  When se intenta el reintento
  Then la fase cambia a "pushing"

Scenario: Disabled detiene el manager
  Given ENGRAM_CLOUD_AUTOSYNC=0
  When el manager detecta el cambio
  Then la fase cambia a "disabled"
  And no se ejecutan más ciclos

Scenario: Reason codes identifican la causa del error
  Given un push falla
  When se consulta el estado
  Then el reason_code debe ser uno de: network_error, auth_error, server_error, timeout, rate_limited

Scenario: Ciclo de sync respeta intervalo configurable
  Given el autosync está en healthy
  When pasan N segundos (default 60)
  Then comienza un nuevo ciclo: pushing → pulling → healthy

Scenario: Pull parcial (incremental) solo trae cambios nuevos
  Given ya se hizo un pull previo
  When se ejecuta el siguiente pull
  Then solo trae entries con updated_at > last_pull_timestamp

Scenario: Estado actual es accesible via API
  Given el autosync manager está corriendo
  When GET /api/cloud/sync-status
  Then retorna: {phase, reason_code, last_push_at, last_pull_at, next_retry_at}
```

## Análisis breve

- **Qué pide realmente:** Background sync manager con state machine (idle, pushing, pulling, healthy, failed, backoff, disabled), reason codes, backoff exponencial, intervalo configurable, status API
- **Módulos sospechados:** `internal/cloud/autosync/` — `manager.go`, `state.go`
- **Riesgos / dependencias:** Goroutine leak si no se detiene correctamente; race conditions en acceso a estado
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:** —
- **Acción derivada:** —
