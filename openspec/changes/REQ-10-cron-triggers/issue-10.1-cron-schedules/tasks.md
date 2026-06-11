# Tasks: issue-10.1-cron-schedules

> Nota de estructura: el proyecto no usa `internal/models/` ni capa
> repository separada — el patrón es service con Pool directo
> (internal/service/cron). Target se modela como `target_type` +
> `target_id` (flow|agent|skill), no flow_slug XOR agent_slug.

## Backend

- [x] Modelo `Cron` → internal/service/cron/service.go (struct con expression, timezone, target, inputs, next_run_at)
- [x] Migración tabla `crons` → migration 000016 con constraints
- [x] Migración tabla `cron_executions` → 000084 (status running/completed/failed/skipped_overlap, duration_ms, índice partial running) — 2026-06-11
- [x] CRUD → Create/GetByID/List/SetEnabled/SoftDelete (List es por org; GetByProjectID N/A: crons son org-scoped)
- [x] `PickDue(limit)` con FOR UPDATE SKIP LOCKED → multi-worker safe; refleja last_run_at/next_run_at post-claim en el retorno — 2026-06-11
- [x] Validación cron expression → robfig/cron v3 (5-field + descriptors @daily/@every)
- [x] Validación timezone IANA → time.LoadLocation
- [x] Cálculo next_run con timezone → NextRun(expression, tz, from)
- [x] Scheduler worker → cronsched.Scheduler ticker 30s (configurable), graceful shutdown por ctx
- [x] Ejecución de flow desde scheduler → dispatchSync target_type=flow (triggerType="cron")
- [x] Ejecución de agente desde scheduler → dispatchSync target_type=agent (+ skill)
- [x] Detección de overlap → StartExecution con INSERT condicional (NOT EXISTS running) en un solo statement; overlap → skipped_overlap sin disparar target — 2026-06-11
- [x] Historial de ejecuciones → StartExecution/FinishExecution/History en executions.go — 2026-06-11
- [x] Handler REST CRUD → POST/GET /api/v1/crons, GET/DELETE /crons/{id} — 2026-06-11
- [x] GET /api/v1/crons/{id}/history → cronHistory (más reciente primero) — 2026-06-11
- [x] PATCH /api/v1/crons/{id} enable/disable → patchCron — 2026-06-11

## Tests

- [x] Parseo expression válida/inválida → cubierto por Create (ErrInvalidCronExpr) + NextRun
- [x] next_run UTC y con timezone → NextRun con time.LoadLocation
- [x] Timezone inválida → ErrInvalidTimezone
- [x] Validación target → ErrInvalidTargetType (whitelist flow|agent|skill, reemplaza XOR de slugs)
- [x] CRUD → cubierto por suite integration del service
- [x] Scheduler ejecuta cron debido → TestCronService_PickDue_PicksDueCron + scheduler_test
- [x] Scheduler salta deshabilitado → TestCronService_PickDue_SkipsDisabled (+ DoesNotPickFuture/Deleted/RespectsLimit)
- [x] Evita doble ejecución → PickDue avanza next_run_at al claim (otro worker no re-pickea)
- [x] Historial se registra → TestCronService_ExecutionHistory_Lifecycle (completed + failed con error y duration) — 2026-06-11
- [x] Overlap skip → TestSabotage_CronService_OverlapSkipped (running activa → skip per-cron, no global) — 2026-06-11
- [x] Sabotaje FOR UPDATE → SKIP LOCKED en PickDue; el sabotaje overlap cubre la doble ejecución a nivel execution

## Cierre

- [x] Verificación ejecución → cubierta por tests integration del scheduler (mismo código)
- [x] Verificación disable → SkipsDisabled
- [x] Suite verde → 2026-06-11 (1037 short + 19 integration cron/scheduler/handlers; snapshots API regenerados)
