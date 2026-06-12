# Tasks: issue-04.10-lifecycle-state-tracking

## Schema

- [x] **lc-001**: Migration `state_machine_rules`
- [x] **lc-002**: Migration `entity_state_transitions` con índices
- [x] **lc-003**: Migration `entity_stuck_flags`
- [x] **lc-004**: Migration agregar `version INT NOT NULL DEFAULT 1` a requirements, issues, intake_payloads, external_sync_state
- [x] **lc-005**: Trigger reject UPDATE/DELETE en entity_state_transitions (o RLS)
- [x] **lc-006**: View `lifecycle_overview`
- [x] **lc-007**: Particionado por mes en entity_state_transitions
- [x] **lc-008**: Seeder state_machine_rules con rules default (intake/req/hu/sync_state)

## Store

- [x] **lc-010**: Package `internal/store/pg/lifecycle/`
- [x] **lc-011**: `RulesStore` (Get, ListByKind, Reload cache)
- [x] **lc-012**: `TransitionsStore.Append`, `Timeline`, `LastTransition`
- [x] **lc-013**: `StuckStore` (UpsertFlag, MarkResolved, ListPending)
- [x] **lc-014**: Query: get entity status with FOR UPDATE
- [x] **lc-015**: Query: bulk lifecycle_overview con paginación
- [x] **lc-016**: Query: metrics aggregations (lead/cycle/throughput)
- [x] **lc-017**: Query: trace graph builder (recursive CTE)

## Service core

- [x] **lc-020**: Package `internal/sdd/lifecycle/`
- [x] **lc-021**: `Service.Transition` (validate rule, check permission, update entity, append audit, publish event)
- [x] **lc-022**: `Service.ForceTransition` (bypass rules con override_reason)
- [x] **lc-023**: `Service.Compensate` (invoke handler + append nueva transition)
- [x] **lc-024**: `Service.GetTimeline`
- [x] **lc-025**: `Service.Trace`
- [x] **lc-026**: `Service.Overview`
- [x] **lc-027**: `Service.Query` (cross-entity filter)
- [x] **lc-028**: `Service.Metrics`
- [x] **lc-029**: Rules cache (in-memory, invalidate via NOTIFY)

## Rules + Conditions

- [x] **lc-030**: `rules.go` evaluator (entity_kind, from, to → required_permission)
- [x] **lc-031**: `conditions.go` evaluator (igualdad de campo en context jsonb)
- [x] **lc-032**: Tabla default seedeada con rules (proposed→approved, approved→in_progress, etc.)

## Compensation handlers

- [x] **lc-040**: Interface `Compensator` por entity_kind
- [x] **lc-041**: Compensator HU (revierte status + signals)
- [x] **lc-042**: Compensator sync_state (delega a sync.Service para PUT issue back)
- [x] **lc-043**: Compensator intake (rollback de REQ/HU/attachments creados — solo si <1h del commit)
- [x] **lc-044**: Marca `context.compensation_partial` cuando handler no puede revertir 100%

## Stuck detector

- [x] **lc-050**: Cron `lifecycle_stuck_cron` cada 1h
- [x] **lc-051**: Rules de stuck configurables (umbral por entity_kind + status)
- [x] **lc-052**: Upsert `entity_stuck_flags` y notify primera vez + escalación 7d
- [x] **lc-053**: Auto-resolve cuando transition cambia estado → trigger marca resolved_at
- [x] **lc-054**: Métrica `lifecycle_stuck_count{kind, status}`

## Refactor: adoptar lifecycle.Transition en otros services

- [x] **lc-060**: `intake.Approve` → llama `lifecycle.Transition('intake', id, 'committed', ...)`
- [x] **lc-061**: `intake.Reject` → `lifecycle.Transition('intake', id, 'rejected', ...)`
- [x] **lc-062**: `sync.Push` éxito → `lifecycle.Transition('sync_state', id, 'ok', ...)` (NEW vs existing manejado en service)
- [x] **lc-063**: `sync.MarkDrift` → `lifecycle.Transition('sync_state', id, 'conflict', ...)`
- [x] **lc-064**: `sync.ResolveConflict` → transitions adecuadas
- [x] **lc-065**: HU CRUD: cualquier UPDATE status → lifecycle.Transition
- [x] **lc-066**: REQ CRUD: idem
- [x] **lc-067**: Webhook handler de sync: status_change vía lifecycle.Transition con actor_kind="webhook"

## Optimistic lock

- [x] **lc-070**: Service.Transition acepta `expected_version` opcional
- [x] **lc-071**: SQL `WHERE version = :expected` returning new version
- [x] **lc-072**: Devuelve 409 con `current_state`, `current_version` si conflict
- [x] **lc-073**: Tests concurrencia (2 transitions mismo id simultáneo)

## Permission scopes

- [x] **lc-080**: Seeder permissions:
      hu:transition:approve, hu:transition:reject, hu:transition:start, hu:transition:complete, hu:transition:archive, hu:transition:block
      intake:transition:* (commit, reject)
      sync_state:transition:* (resolve)
      lifecycle:force_transition, lifecycle:compensate
- [x] **lc-081**: Asignar a roles default (PM, dev, viewer)
- [x] **lc-082**: Permission check antes de cada Transition

## Eventos

- [x] **lc-090**: Publisher `lifecycle.transition` con payload completo
- [x] **lc-091**: Publisher `lifecycle.stuck_detected`
- [x] **lc-092**: Publisher `lifecycle.force_transition` (alerta seguridad)
- [x] **lc-093**: Publisher `lifecycle.compensated`

## Notifications

- [x] **lc-100**: Suscriber a `lifecycle.stuck_detected` → email + slack al owner
- [x] **lc-101**: Suscriber a `lifecycle.force_transition` → notif admin (audit)

## MCP tools

- [x] **lc-110**: `domain_lifecycle_transition`
- [x] **lc-111**: `domain_lifecycle_force_transition`
- [x] **lc-112**: `domain_lifecycle_compensate`
- [x] **lc-113**: `domain_lifecycle_timeline`
- [x] **lc-114**: `domain_lifecycle_trace`
- [x] **lc-115**: `domain_lifecycle_overview`
- [x] **lc-116**: `domain_lifecycle_query`
- [x] **lc-117**: `domain_lifecycle_metrics`

## Métricas / Tracing

- [x] **lc-120**: Counter `lifecycle_transitions_total{entity_kind, from, to, result}`
- [x] **lc-121**: Histogram `lifecycle_transition_duration_seconds{entity_kind}`
- [x] **lc-122**: Counter `lifecycle_force_transitions_total{entity_kind}`
- [x] **lc-123**: Gauge `lifecycle_stuck_count{entity_kind, status}`
- [x] **lc-124**: Histogram `lifecycle_lead_time_seconds{kind}` p50/p90
- [x] **lc-125**: Tracing spans `lifecycle.transition` con atributos kind, id, from, to
- [x] **lc-126**: Logs estructurados con transition_id

## Tests

- [x] **lc-200**: Unit rule evaluator
- [x] **lc-201**: Unit conditions evaluator
- [x] **lc-202**: Unit metrics calculator con seed
- [x] **lc-203**: Integration Transition válida + invalida
- [x] **lc-204**: Integration permission denied → 403
- [x] **lc-205**: Integration concurrent transition → 409
- [x] **lc-206**: Integration ForceTransition bypass + audit
- [x] **lc-207**: Integration Compensate completo + parcial
- [x] **lc-208**: Integration stuck detector con seed de 3 stuck entities
- [x] **lc-209**: Integration stuck escalation a 7d
- [x] **lc-210**: Integration auto-resolve cuando transition
- [x] **lc-211**: Integration cross-tenant: org A no ve transitions de org B
- [x] **lc-212**: Integration trace cross-entity
- [x] **lc-213**: Integration metrics lead_time/cycle_time over 100 seeds
- [x] **lc-214**: Integration overview con 50 REQs + filter
- [x] **lc-215**: Sabotaje UPDATE entity_state_transitions → falla
- [x] **lc-216**: Sabotaje compensación handler crash → marca compensation_partial

## Documentación

- [x] **lc-300**: `docs/lifecycle/overview.md` con state machine diagrams por entity
- [x] **lc-301**: `docs/lifecycle/permissions.md` mapa de scopes → roles
- [x] **lc-302**: `docs/lifecycle/metrics.md` definiciones lead/cycle/throughput
- [x] **lc-303**: Runbook `docs/runbooks/stuck-entities.md` cómo desbloquear
- [x] **lc-304**: Runbook `docs/runbooks/compensate-bad-push.md` cómo revertir push a Jira erróneo
