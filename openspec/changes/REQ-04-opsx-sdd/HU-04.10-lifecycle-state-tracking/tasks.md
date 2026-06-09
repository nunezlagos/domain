# Tasks: HU-04.10-lifecycle-state-tracking

## Schema

- [ ] **lc-001**: Migration `state_machine_rules`
- [ ] **lc-002**: Migration `entity_state_transitions` con índices
- [ ] **lc-003**: Migration `entity_stuck_flags`
- [ ] **lc-004**: Migration agregar `version INT NOT NULL DEFAULT 1` a requirements, user_stories, intake_payloads, external_sync_state
- [ ] **lc-005**: Trigger reject UPDATE/DELETE en entity_state_transitions (o RLS)
- [ ] **lc-006**: View `lifecycle_overview`
- [ ] **lc-007**: Particionado por mes en entity_state_transitions
- [ ] **lc-008**: Seeder state_machine_rules con rules default (intake/req/hu/sync_state)

## Store

- [ ] **lc-010**: Package `internal/store/pg/lifecycle/`
- [ ] **lc-011**: `RulesStore` (Get, ListByKind, Reload cache)
- [ ] **lc-012**: `TransitionsStore.Append`, `Timeline`, `LastTransition`
- [ ] **lc-013**: `StuckStore` (UpsertFlag, MarkResolved, ListPending)
- [ ] **lc-014**: Query: get entity status with FOR UPDATE
- [ ] **lc-015**: Query: bulk lifecycle_overview con paginación
- [ ] **lc-016**: Query: metrics aggregations (lead/cycle/throughput)
- [ ] **lc-017**: Query: trace graph builder (recursive CTE)

## Service core

- [ ] **lc-020**: Package `internal/sdd/lifecycle/`
- [ ] **lc-021**: `Service.Transition` (validate rule, check permission, update entity, append audit, publish event)
- [ ] **lc-022**: `Service.ForceTransition` (bypass rules con override_reason)
- [ ] **lc-023**: `Service.Compensate` (invoke handler + append nueva transition)
- [ ] **lc-024**: `Service.GetTimeline`
- [ ] **lc-025**: `Service.Trace`
- [ ] **lc-026**: `Service.Overview`
- [ ] **lc-027**: `Service.Query` (cross-entity filter)
- [ ] **lc-028**: `Service.Metrics`
- [ ] **lc-029**: Rules cache (in-memory, invalidate via NOTIFY)

## Rules + Conditions

- [ ] **lc-030**: `rules.go` evaluator (entity_kind, from, to → required_permission)
- [ ] **lc-031**: `conditions.go` evaluator (igualdad de campo en context jsonb)
- [ ] **lc-032**: Tabla default seedeada con rules (proposed→approved, approved→in_progress, etc.)

## Compensation handlers

- [ ] **lc-040**: Interface `Compensator` por entity_kind
- [ ] **lc-041**: Compensator HU (revierte status + signals)
- [ ] **lc-042**: Compensator sync_state (delega a sync.Service para PUT issue back)
- [ ] **lc-043**: Compensator intake (rollback de REQ/HU/attachments creados — solo si <1h del commit)
- [ ] **lc-044**: Marca `context.compensation_partial` cuando handler no puede revertir 100%

## Stuck detector

- [ ] **lc-050**: Cron `lifecycle_stuck_cron` cada 1h
- [ ] **lc-051**: Rules de stuck configurables (umbral por entity_kind + status)
- [ ] **lc-052**: Upsert `entity_stuck_flags` y notify primera vez + escalación 7d
- [ ] **lc-053**: Auto-resolve cuando transition cambia estado → trigger marca resolved_at
- [ ] **lc-054**: Métrica `lifecycle_stuck_count{kind, status}`

## Refactor: adoptar lifecycle.Transition en otros services

- [ ] **lc-060**: `intake.Approve` → llama `lifecycle.Transition('intake', id, 'committed', ...)`
- [ ] **lc-061**: `intake.Reject` → `lifecycle.Transition('intake', id, 'rejected', ...)`
- [ ] **lc-062**: `sync.Push` éxito → `lifecycle.Transition('sync_state', id, 'ok', ...)` (NEW vs existing manejado en service)
- [ ] **lc-063**: `sync.MarkDrift` → `lifecycle.Transition('sync_state', id, 'conflict', ...)`
- [ ] **lc-064**: `sync.ResolveConflict` → transitions adecuadas
- [ ] **lc-065**: HU CRUD: cualquier UPDATE status → lifecycle.Transition
- [ ] **lc-066**: REQ CRUD: idem
- [ ] **lc-067**: Webhook handler de sync: status_change vía lifecycle.Transition con actor_kind="webhook"

## Optimistic lock

- [ ] **lc-070**: Service.Transition acepta `expected_version` opcional
- [ ] **lc-071**: SQL `WHERE version = :expected` returning new version
- [ ] **lc-072**: Devuelve 409 con `current_state`, `current_version` si conflict
- [ ] **lc-073**: Tests concurrencia (2 transitions mismo id simultáneo)

## Permission scopes

- [ ] **lc-080**: Seeder permissions:
      hu:transition:approve, hu:transition:reject, hu:transition:start, hu:transition:complete, hu:transition:archive, hu:transition:block
      intake:transition:* (commit, reject)
      sync_state:transition:* (resolve)
      lifecycle:force_transition, lifecycle:compensate
- [ ] **lc-081**: Asignar a roles default (PM, dev, viewer)
- [ ] **lc-082**: Permission check antes de cada Transition

## Eventos

- [ ] **lc-090**: Publisher `lifecycle.transition` con payload completo
- [ ] **lc-091**: Publisher `lifecycle.stuck_detected`
- [ ] **lc-092**: Publisher `lifecycle.force_transition` (alerta seguridad)
- [ ] **lc-093**: Publisher `lifecycle.compensated`

## Notifications

- [ ] **lc-100**: Suscriber a `lifecycle.stuck_detected` → email + slack al owner
- [ ] **lc-101**: Suscriber a `lifecycle.force_transition` → notif admin (audit)

## MCP tools

- [ ] **lc-110**: `domain_lifecycle_transition`
- [ ] **lc-111**: `domain_lifecycle_force_transition`
- [ ] **lc-112**: `domain_lifecycle_compensate`
- [ ] **lc-113**: `domain_lifecycle_timeline`
- [ ] **lc-114**: `domain_lifecycle_trace`
- [ ] **lc-115**: `domain_lifecycle_overview`
- [ ] **lc-116**: `domain_lifecycle_query`
- [ ] **lc-117**: `domain_lifecycle_metrics`

## Métricas / Tracing

- [ ] **lc-120**: Counter `lifecycle_transitions_total{entity_kind, from, to, result}`
- [ ] **lc-121**: Histogram `lifecycle_transition_duration_seconds{entity_kind}`
- [ ] **lc-122**: Counter `lifecycle_force_transitions_total{entity_kind}`
- [ ] **lc-123**: Gauge `lifecycle_stuck_count{entity_kind, status}`
- [ ] **lc-124**: Histogram `lifecycle_lead_time_seconds{kind}` p50/p90
- [ ] **lc-125**: Tracing spans `lifecycle.transition` con atributos kind, id, from, to
- [ ] **lc-126**: Logs estructurados con transition_id

## Tests

- [ ] **lc-200**: Unit rule evaluator
- [ ] **lc-201**: Unit conditions evaluator
- [ ] **lc-202**: Unit metrics calculator con seed
- [ ] **lc-203**: Integration Transition válida + invalida
- [ ] **lc-204**: Integration permission denied → 403
- [ ] **lc-205**: Integration concurrent transition → 409
- [ ] **lc-206**: Integration ForceTransition bypass + audit
- [ ] **lc-207**: Integration Compensate completo + parcial
- [ ] **lc-208**: Integration stuck detector con seed de 3 stuck entities
- [ ] **lc-209**: Integration stuck escalation a 7d
- [ ] **lc-210**: Integration auto-resolve cuando transition
- [ ] **lc-211**: Integration cross-tenant: org A no ve transitions de org B
- [ ] **lc-212**: Integration trace cross-entity
- [ ] **lc-213**: Integration metrics lead_time/cycle_time over 100 seeds
- [ ] **lc-214**: Integration overview con 50 REQs + filter
- [ ] **lc-215**: Sabotaje UPDATE entity_state_transitions → falla
- [ ] **lc-216**: Sabotaje compensación handler crash → marca compensation_partial

## Documentación

- [ ] **lc-300**: `docs/lifecycle/overview.md` con state machine diagrams por entity
- [ ] **lc-301**: `docs/lifecycle/permissions.md` mapa de scopes → roles
- [ ] **lc-302**: `docs/lifecycle/metrics.md` definiciones lead/cycle/throughput
- [ ] **lc-303**: Runbook `docs/runbooks/stuck-entities.md` cómo desbloquear
- [ ] **lc-304**: Runbook `docs/runbooks/compensate-bad-push.md` cómo revertir push a Jira erróneo
