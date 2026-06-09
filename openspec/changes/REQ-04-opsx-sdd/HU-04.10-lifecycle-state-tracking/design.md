# Design: HU-04.10-lifecycle-state-tracking

## Decisión arquitectónica

**Audit immutable centralizado**. Cada cambio de estado de cualquier entidad SDD (intake/req/hu/sync) se persiste en `entity_state_transitions` con metadata mínima (kind, id, from, to, actor, time, reason). NUNCA UPDATE ni DELETE — sólo INSERT (enforced con role + RLS).

**State machine declarativa en BD**, no en código. Tabla `state_machine_rules` define `(entity_kind, from_state, to_state) → required_permission`. Permite ajustar reglas sin redeploy. Seedeado vía HU-01.7.

**Sin reactor pattern overcomplicado**. El servicio `lifecycle.Transition()` es la **única puerta** para cambiar status de las entidades SDD. Todos los otros services (intake.Approve, sync.ResolveConflict, hu CRUD) lo llaman en lugar de UPDATE directo. Esto evita transiciones inconsistentes.

**Stuck detector como cron periódico**, no listener evento. Cada 1h corre query de "rule violated" y emite events si encuentra. Simple, robusto, no requiere ordering perfecto.

**View materialized opcional**. `lifecycle_overview` es VIEW estándar en MVP. Si performance no alcanza (>10k REQs), se promueve a MATERIALIZED VIEW con refresh cron.

## Componentes

```
internal/sdd/lifecycle/
  service.go              # Transition, ForceTransition, Compensate, GetTimeline, GetOverview, Query, Metrics, Trace
  rules.go                # eval reglas state_machine_rules
  conditions.go           # eval expressions simples (igualdad de campo)
  stuck.go                # cron detector + notify
  metrics.go              # lead_time, cycle_time, throughput calculators
  trace.go                # cross-entity graph builder
  events.go               # publish lifecycle.transition, lifecycle.stuck_detected, lifecycle.force_transition

internal/store/pg/lifecycle/
  rules_store.go          # CRUD state_machine_rules (read-mostly)
  transitions_store.go    # AppendTransition (insert-only), GetTimeline, ListByActor
  stuck_store.go          # CRUD entity_stuck_flags

internal/mcp/tools/lifecycle/
  overview.go, timeline.go, trace.go, query.go, metrics.go,
  force_transition.go, compensate.go, transition.go
```

## API del servicio

```go
type Service interface {
    // Núcleo: transition validada
    Transition(ctx, kind, id, toState, opts) (*Transition, error)
    // opts: { reason, context, expected_version (optimistic lock), source_event_id }

    // Admin override
    ForceTransition(ctx, kind, id, toState, overrideReason) (*Transition, error)

    // Compensation
    Compensate(ctx, transitionID, reason) (*Transition, error)

    // Queries
    GetTimeline(ctx, kind, id) ([]Transition, error)
    Trace(ctx, startKind, startID) (*Graph, error)
    Overview(ctx, orgID, filter) ([]ReqOverview, error)
    Query(ctx, filter) ([]EntityRef, error)
    Metrics(ctx, orgID, period) (*LifecycleMetrics, error)
}
```

`Transition` es la **única función llamada por los demás services** cuando cambian status. Ejemplo: `intake.Approve` que antes hacía `UPDATE intake_payloads SET status='committed'` ahora hace:

```go
return lifecycle.Transition(ctx, "intake", intakeID, "committed",
    TransitionOpts{Reason: "human approved create_new", Context: {hu_id, req_id}})
```

Y el servicio internamente:
1. Lee row entity con `SELECT ... FOR UPDATE`.
2. Calcula `from_state` actual.
3. Verifica regla `(intake, pending_review, committed)` en state_machine_rules.
4. Verifica permission del actor.
5. Verifica `expected_version` si proveída.
6. Update entity status + version++.
7. Insert en entity_state_transitions.
8. Publish event lifecycle.transition.
9. Commit transaction.

## Stuck detector

Cron cada 1h con rules:

```sql
-- intake pending_review > 7d
SELECT 'intake', id, status, updated_at FROM intake_payloads
WHERE status = 'pending_review' AND updated_at < now() - interval '7 days';

-- hu in_progress > 30d
SELECT 'hu', id, status, last_transition FROM user_stories
LEFT JOIN (SELECT entity_id, max(occurred_at) as last_transition
           FROM entity_state_transitions WHERE entity_kind='hu' GROUP BY entity_id) t
ON t.entity_id = user_stories.id
WHERE status='in_progress' AND coalesce(last_transition, created_at) < now() - interval '30 days';

-- sync conflict > 3d
SELECT 'sync_state', id, sync_status, drift_detected_at FROM external_sync_state
WHERE sync_status='conflict' AND drift_detected_at < now() - interval '3 days';
```

Cada hit upsert en `entity_stuck_flags`. Si era nuevo (no había row) → notifica owner + admin via REQ-20. Si ya existía y `notified_at < now() - 7d` → notifica de nuevo (escalación).

Resolución automática: cuando una transition cambia el estado de la entidad, un trigger marca `entity_stuck_flags.resolved_at = now()`.

## Métricas lifecycle

Calculadas on-demand (no precomputadas en MVP):

```sql
-- Lead time (intake → hu.done)
SELECT
  percentile_cont(0.5) WITHIN GROUP (ORDER BY age_seconds) as p50,
  percentile_cont(0.9) WITHIN GROUP (ORDER BY age_seconds) as p90
FROM (
  SELECT EXTRACT(EPOCH FROM (hu_done_at - intake_committed_at)) as age_seconds
  FROM ...joins...
  WHERE hu.status='done' AND hu.done_at > now() - period
) sub;

-- Cycle time por estado
SELECT
  to_state,
  AVG(EXTRACT(EPOCH FROM (next_occurred_at - occurred_at))) as avg_seconds_in_state
FROM (
  SELECT *, lead(occurred_at) OVER (PARTITION BY entity_id ORDER BY occurred_at) as next_occurred_at
  FROM entity_state_transitions
  WHERE entity_kind='hu' AND occurred_at > now() - period
) sub
GROUP BY to_state;
```

Si volumen lo justifica, materialized view + refresh nightly.

## Compensation pattern

Cuando una transition tiene side-effect (push a Jira), `Compensate(transitionID, reason)`:

1. Lee la transition original.
2. Inspecciona `context` para conocer side-effects (ej. `{jira_pushed: true, key: "DIDE-145"}`).
3. Invoca handler de compensation (driver Jira → PUT issue volviendo al estado anterior).
4. Si compensation OK → inserta nueva transition con `compensates_id=originalID`.
5. Si compensation parcial → marca `context.compensation_partial=true`, alerta admin.

Compensación es **append-only**: NUNCA borra la transition original. Esto preserva audit. UI muestra ambas con flecha "compensated by ...".

## Optimistic locking

Cada entidad SDD agrega columna `version INT NOT NULL DEFAULT 1`. Transition `WHERE id=:id AND version=:expected` → `RETURNING version+1`. Si row count = 0 → conflict, devuelve 409 con `current_state`, `current_version`.

## Permission model

Permisos granulares definidos:
- `hu:transition:approve` (proposed→approved)
- `hu:transition:reject` (proposed|approved→rejected)
- `hu:transition:start` (approved→in_progress)
- `hu:transition:complete` (in_progress→done)
- `hu:transition:archive` (done→archived)
- `hu:transition:block` (in_progress→blocked)
- Análogo para intake/req/sync_state
- `lifecycle:force_transition` (admin override)
- `lifecycle:compensate` (admin)

Asignados a roles default vía REQ-02.2 (PM tiene todos los `transition:*`, dev tiene `start/complete/block`, viewer ninguno).

## Trace graph

`Trace(startKind, startID)` arma DFS de relaciones:

```
intake_to_req_links: intake → req, intake → hu
user_stories: hu → req (FK)
external_sync_state: sync → req/hu (polymorphic)
attachments: attachment → hu (FK)
intake_payloads.resulting_*_id: intake → req/hu
```

Devuelve grafo con nodos `{kind, id, current_state, key_fields, last_transition_at}` y aristas `{from, to, relationship_type}`.

## Trade-offs

| Decisión | Trade-off |
|---|---|
| Single audit table polymorphic | Indexing eficiente con (entity_kind, entity_id) ; trade: queries específicas por kind son menos rápidas que tabla dedicada |
| Rules en BD | Flexibilidad; trade: 1 query extra por transition (cacheable en RAM) |
| Append-only (no update audit) | GDPR right-to-erasure complica → solución: tomb `actor_id` con hash en lugar de borrar (HU-04.10 ↔ REQ-23.4) |
| Stuck detector cron 1h | Latency hasta 1h en detección; trade: simple. Mejorable con LISTEN evento de inactividad |
| Lock pesimista en transition | Bloqueo breve durante la transition; trade: garantiza no race. Optimistic via version es alternativa más liviana ya implementada |
| Sin DSL completo en conditions | Lo necesario es igualdad simple; complicado puede meterse al hardcoded handler por ahora |
| Compensation manual (no automática) | Más friction; trade: side effects son sensibles, mejor humano confirma |

## Tests críticos

- Transition válida: persiste + emite evento
- Transition inválida: rechazada con error
- Transition sin permission: 403
- Optimistic lock conflict: 409
- Force transition: bypasses rules, persiste con flag
- Compensate: invoca handler, crea nueva transition con compensates_id
- Stuck detector: hits 3 entidades stuck, notif una vez, escalación a los 7d
- Cross-entity trace: intake → req → hu → sync → attachments todos en el grafo
- Metrics: lead_time p50/p90 calculadas correcto sobre seed data
- Cross-tenant scope: org A no ve transitions de org B
