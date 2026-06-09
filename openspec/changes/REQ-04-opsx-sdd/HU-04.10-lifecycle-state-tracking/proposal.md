# Proposal: HU-04.10-lifecycle-state-tracking

## Intención

Capa unificada de state machine + audit immutable + métricas de lifecycle sobre las entidades SDD (intake, REQ, HU, sync_state). Centralizar las transiciones de estado en un único servicio (`lifecycle.Transition`) que aplica reglas declarativas, valida permisos, mantiene optimistic lock, persiste audit append-only, publica eventos y permite ver dashboards de progreso / bottlenecks / stuck items.

## Scope

**Incluye:**
- Tabla `state_machine_rules` (declarativa, seedeada via HU-01.7)
- Tabla `entity_state_transitions` (audit immutable, polymorphic entity)
- Tabla `entity_stuck_flags` (detección stuck + tracking de notificación)
- View `lifecycle_overview` agregada por REQ
- Servicio `lifecycle.Service` con `Transition`, `ForceTransition`, `Compensate`, `GetTimeline`, `Trace`, `Overview`, `Query`, `Metrics`
- Adopción del servicio por intake.Approve, sync.Resolve, hu CRUD, req CRUD (refactor de inserts/updates de status existentes)
- Optimistic locking via columna `version` en cada entidad SDD
- Cron stuck detector cada 1h con notify (REQ-20)
- Métricas lead_time/cycle_time/throughput calculadas on-demand
- Cross-entity trace (intake → REQ → HU → sync → attachments)
- 8 MCP tools (overview, timeline, trace, query, metrics, force_transition, compensate, transition)
- Permission scopes granulares por transición (REQ-02.2 RBAC)
- Eventos al event bus: lifecycle.transition, lifecycle.stuck_detected, lifecycle.force_transition, lifecycle.compensated
- Compensation pattern para revertir transitions con side effects (append nueva transition, no UPDATE)
- Multi-tenant scope estricto

**No incluye:**
- DSL completo para reglas (igualdad simple suficiente en MVP)
- Predictive analytics (cuándo se va a terminar un HU)
- Visualizaciones (parte de REQ-16 Web UI)
- Sprint/iteration tracking (Jira lo cubre, Domain no replica)
- Materialized view auto-refresh (manual o cron si necesario)

## Alternativas consideradas

### A. State machine hardcoded en Go (sin tabla rules)

**Por qué no:** cambios requieren redeploy. PMs cliente piden ajustes ad-hoc ("agreguen estado 'en_qa' entre approved e in_progress") → con BD se hace con un INSERT.

### B. Sin tabla audit (solo timestamps en entity table)

**Por qué no:** se pierde quién/cuándo/por_qué de cada transición. Auditoría regulatoria + GDPR exige saber el actor de cada cambio.

### C. Audit por entidad (3 tablas: intake_transitions, hu_transitions, sync_transitions)

**Por qué no:** queries cross-entity costosas; misma estructura repetida 3 veces. Polymorphic con índice (kind, id) escala bien para volúmenes esperados.

### D. UPDATE permitido sobre audit table

**Por qué no:** invalida auditoría. Append-only enforcement con role + RLS.

### E. Workflow engine externo (Temporal, Camunda)

**Por qué no en MVP:** overkill para 4 entidades con pocos estados. Si la lógica de workflow crece (multi-step transitions con compensation paralela), considerar REQ-09 flow-system para esos casos específicos.

### F. Stuck detection vía LISTEN en cambios

**Por qué no:** complejo (qué hacer con un evento "cambió hace 6 días que era stuck"?). Cron periódico es simple, idempotente y suficiente para latencia tolerable.

### G. Compensation automática

**Por qué no MVP:** revertir un push a Jira con N attachments y comments mid-flight es delicado. Humano confirma compensation y observa que side effects revertidos OK.

## Dependencias

**Hard:**
- HU-04.1 (REQs CRUD), HU-04.2 (HUs CRUD)
- HU-04.8 (intake states)
- HU-04.9 (sync states)
- REQ-01 (DB schema)
- REQ-02.2 (RBAC para permission check)
- REQ-02.4 (audit log entries por transition)
- HU-01.7 (seeders para state_machine_rules y permissions)

**Soft:**
- REQ-10.3 (event bus para publish lifecycle.*)
- REQ-20 (notifications stuck)
- REQ-17 (observability — métricas + traces)
- REQ-16 (Web UI consumidor)
- REQ-15 (cost observability — analytics derivado)

## Plan de release

1. Schema rules + transitions + stuck_flags + version columns + view
2. Service.Transition con validation simple + audit insert
3. Refactor intake.Approve, sync.Resolve, hu/req CRUD para llamar Transition
4. Optimistic lock + 409 handling
5. ForceTransition + permission scopes
6. Compensate handlers por entity kind
7. Stuck detector cron + notif
8. Timeline + Trace + Overview + Query queries
9. Metrics calculator
10. MCP tools

## Riesgos

| Riesgo | Mitigación |
|---|---|
| Refactor masivo de status updates en services existentes | Hacer 1 service a la vez con tests dedicated; deploy incremental |
| Audit table crece rápido (1M+ rows/año) | Particionado por created_at (mensual); archive >2 años a cold storage |
| Stuck false positives (HU "in_progress" 30d pero correcto) | Permitir override por entity con field `stuck_exception_until` |
| Permission map demasiado granular | Empezar con 5 default (transition:*) + custom roles en HU futura |
| Optimistic lock retries → race storm | Backoff exponencial en client retry; máx 3 reintentos |
| Compensation incompleta deja state inconsistente | Marcar `context.compensation_partial`, alerta admin, manual cleanup tool |

## Tests críticos

- Transition válida + invalid (rechazada con error claro)
- Sin permission → 403
- Concurrent transition mismo HU → 1 succeeds, 1 returns 409
- ForceTransition bypass rules, persiste con flag forced=true
- Compensate crea nueva transition vinculada
- Cron detecta 3 stuck → notif una vez → notif again a 7d
- Resolve auto: transition → marca stuck.resolved_at
- Cross-tenant scope: org A no ve transitions de org B
- Lead time calculation con 100 HUs seedeadas
- Trace grafo intake→hu→sync→attachments completo
- Audit append-only: UPDATE on entity_state_transitions falla (RLS o trigger)
