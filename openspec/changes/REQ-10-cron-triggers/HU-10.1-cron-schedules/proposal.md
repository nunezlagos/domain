# Proposal: HU-10.1-cron-schedules

## Intención

CRUD de cron schedules con evaluación periódica (cada minuto) que dispara ejecución de flows o agentes. Soporte de timezones, cálculo de next_run, historial de ejecuciones y manejo de overlaps.

## Scope

**Incluye:**
- Modelo `Cron` con campos: name, cron_expression, flow_slug (nullable), agent_slug (nullable), project_id, timezone, enabled, params (jsonb), last_run, next_run
- CRUD REST de crons
- Validación de expresión cron (biblioteca `robfig/cron`)
- Validación de timezone (IANA timezone database)
- Scheduler worker: loop cada 60s, consulta crons con next_run ≤ now, ejecuta y actualiza
- Historial de ejecuciones (tabla `cron_executions`)
- Manejo de overlaps: skip si ejecución anterior sigue en progreso (opcional: cola)
- Re-cálculo de next_run en create/update/enable

**Excluye:**
- Cron con segundos (formato estándar de 5 campos es suficiente)
- Notificaciones de cron fallido (puede integrarse con sistema de alertas en REQ-15)
- Soporte para expresiones no estándar (@daily, @hourly) — se puede agregar después

## Enfoque técnico

- Librería `github.com/robfig/cron/v3` para parseo y cálculo de próximas ejecuciones
- Scheduler worker: goroutine con `time.Ticker` cada 60s
  - Query: `SELECT * FROM crons WHERE enabled = true AND next_run <= NOW()`
  - Para cada cron: lanzar ejecución de flow o agente en goroutine separada
  - Actualizar last_run y next_run en transacción atómica
  - Insertar registro en cron_executions
- Timezone: `time.LoadLocation` para convertir cron evaluation al timezone local
- Para evitar doble ejecución: usar `SELECT ... FOR UPDATE SKIP LOCKED` en Postgres
- Historial: tabla `cron_executions` con FK a cron y domain_flow_run/domain_agent_run

## Riesgos

- Doble ejecución si scheduler se ejecuta más de una vez → usar FOR UPDATE SKIP LOCKED + idempotencia
- Overlap: si un cron tarda 5 min pero corre cada minuto, puede saturar → skip if previous still running
- Timezone changes (DST): `robfig/cron` maneja DST automáticamente con timezone locations

## Testing

- Unit: parseo de expresión cron válida e inválida
- Unit: cálculo de next_run con y sin timezone
- Unit: validación de campos mutuamente excluyentes (flow vs agent)
- Integration: CRUD contra DB
- Integration: scheduler worker ejecuta cron debido
- Integration: cron deshabilitado no se ejecuta
- Integration: historial de ejecuciones
- Sabotaje: query sin FOR UPDATE → test de doble ejecución falla
