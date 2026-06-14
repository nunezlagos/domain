# issue-04.10-lifecycle-state-tracking

**Origen:** `REQ-04-opsx-sdd`
**Prioridad tentativa:** media-alta
**Tipo:** feature

## Historia de usuario

**Como** PM/desarrollador que coordina múltiples REQs e HUs en distintas etapas (intake, draft, review, in_progress, done, archived) con su mirror en Jira
**Quiero** un view unificado del lifecycle de cada entidad (intake, REQ, HU, sync_state) con state machine explícita, transiciones auditadas, y reportes agregados (cuánto tarda algo en intake→done, qué está stuck, qué tiene drift no resuelto)
**Para** que pueda detectar bloqueos (HUs paradas en "pending_review" >7 días), tomar decisiones de cierre de sprint, justificar tiempos al cliente, y que ningún ítem se pierda en el limbo entre Domain y Jira

## Diferencia con tablas existentes

issue-04.8 trackea `intake_payloads.status`. issue-04.9 trackea `external_sync_state.sync_status`. Las HUs y REQs tienen su `status` propio. Esta HU **unifica** todas en un view consultable + agrega:

1. **State machines explícitas** con tabla `entity_state_transitions` (audit immutable de cada cambio de estado).
2. **Reglas de transición declarativas** (qué transiciones son permitidas por entidad + rol).
3. **Lifecycle metrics** (timing entre estados, leadtime, bottlenecks).
4. **Stuck detector** (cron que detecta entidades sin transición por > X tiempo y notifica).
5. **Dashboard query** (1 SQL view para Web UI / CLI).
6. **Vinculación cross-entity** (un intake → REQ → HU → sync_state, todo trazable en 1 query).

## Criterios de aceptación

### Escenario 1: Cada cambio de estado se audita

```gherkin
Dado que existe un user_story con status="proposed"
Cuando humano (o evento) cambia a "approved"
Entonces se inserta row en entity_state_transitions con:
  - entity_kind="hu", entity_id=...
  - from_state="proposed", to_state="approved"
  - actor_kind="user"|"agent"|"system", actor_id, actor_name
  - reason (opcional), context (opcional jsonb)
  - occurred_at NOW(), tx_id (para tracking)
Y NUNCA se permite UPDATE/DELETE sobre entity_state_transitions (immutable)
```

### Escenario 2: Transiciones inválidas se rechazan

```gherkin
Dado que existe state machine declarativa para entidad "hu":
  proposed → approved | rejected
  approved → in_progress | rejected
  in_progress → done | blocked
  blocked → in_progress | rejected
  done → archived
  rejected → (terminal)
Cuando se intenta transicionar "done" → "in_progress"
Entonces servicio rechaza con error `invalid_transition` y NO se persiste
Y se loguea intento
```

### Escenario 3: Transiciones permitidas por rol

```gherkin
Dado que tabla state_machine_rules define `transition` x `required_permission`
Cuando user con permission "hu:edit" intenta transicionar HU "approved" → "in_progress"
Entonces se permite (rule cumple)
Cuando user sin permission intenta "approved" → "rejected"
Entonces 403 con `missing_permission: hu:reject`
```

### Escenario 4: Dashboard query unificada

```gherkin
Dado que existe view `lifecycle_overview` (materialized opcional)
Cuando se consulta `domain_lifecycle_overview({organization_id, filter?})`
Entonces se devuelve por cada REQ:
  - req.slug, req.status, req.created_at
  - count(HUs) por status (proposed/approved/in_progress/done/archived/rejected)
  - count(intakes asociados) por intake_status
  - count(sync_states) por sync_status
  - lead_time_days_p50, p90 (desde intake → hu.done)
  - bottleneck_state (donde se pasa más tiempo)
  - last_activity_at
  - drift_count (sync_status=conflict)
Y soporta filter por status/dateRange/sourceIntake/hasJiraSync/hasDrift
```

### Escenario 5: Stuck detector cron

```gherkin
Dado que entidad lleva > umbral configurado en mismo estado:
  intake_payloads.status="pending_review" > 7 días
  issues.status="in_progress" > 30 días sin commit
  external_sync_state.sync_status="conflict" > 3 días
Cuando worker cron corre (cada 1h)
Entonces inserta evento "lifecycle.stuck_detected" con detalles
Y notifica al owner + admin (REQ-20)
Y persiste flag `stuck_since` en la entidad para que la UI lo destaque
```

### Escenario 6: Timeline de una entidad

```gherkin
Dado que existe HU con varias transiciones de estado a lo largo del tiempo
Cuando se consulta `domain_lifecycle_timeline({entity_kind: "hu", entity_id})`
Entonces devuelve cronología:
  [
    {at: T0, from: null, to: "proposed", actor: "agent-claude", reason: "from intake INT-123"},
    {at: T1, from: "proposed", to: "approved", actor: "user-mauricio", reason: "review OK"},
    {at: T2, from: "approved", to: "in_progress", actor: "user-bruno"},
    {at: T3, from: "in_progress", to: "done", actor: "system", reason: "jira webhook DIDE-145 → Done"}
  ]
Y enriquecida con: commits git asociados (vía external links), events del sync_state, comments
```

### Escenario 7: Force transition (admin override)

```gherkin
Dado que existe entidad bloqueada en estado terminal por error humano (ej. archived prematuramente)
Cuando admin invoca `domain_lifecycle_force_transition({entity_kind, entity_id, to_state, override_reason})`
Entonces NO se valida state_machine_rules
Y se persiste transition con flag `forced=true`, `override_reason` obligatorio
Y se inserta entry de audit con severity="warning"
Y emite evento "lifecycle.force_transition" (visible en dashboards de seguridad)
```

### Escenario 8: Lifecycle metrics agregados

```gherkin
Dado que existen N HUs cerradas en último trimestre
Cuando se consulta `domain_lifecycle_metrics({organization_id, period})`
Entonces devuelve:
  - throughput: HUs done por semana
  - lead_time_p50/p90 desde intake → done
  - cycle_time por etapa (proposed→approved, approved→in_progress, in_progress→done)
  - rejection_rate
  - drift_resolution_avg_hours
  - stuck_count actual
  - top_bottleneck_state
Y filtrable por tag/audience/req/sprint(*)
```

### Escenario 9: Vinculación cross-entity en una sola query

```gherkin
Dado que existe intake INT-123 → issue-12.3 → sync con DIDE-145
Cuando se consulta `domain_lifecycle_trace({start_kind: "intake", start_id: "INT-123"})`
Entonces se devuelve grafo:
  intake(INT-123, status=committed)
    → req(REQ-12, status=active)
      → hu(issue-12.3, status=in_progress)
        → sync(jira:DIDE-145, sync_status=ok)
        → attachments(2 files in S3)
Y cada nodo trae su current status + last_transition_at
```

### Escenario 10: Estado consistente con sub-states

```gherkin
Dado que un REQ tiene status="active" pero ningún HU está in_progress
Y otro REQ tiene status="active" con 3 HUs in_progress
Cuando se calcula `derived_state`
Entonces el primer REQ se marca "idle" (active pero sin movimiento)
Y el segundo se marca "live" (active + HUs avanzando)
Y la UI puede usar derived_state para destacar visualmente
```

### Escenario 11: Bulk view por status

```gherkin
Dado que humano quiere ver "todo lo que está stuck"
Cuando consulta `domain_lifecycle_query({state: "stuck", since?: "7d"})`
Entonces devuelve lista cross-entity:
  [
    {kind: "intake", id, status: "pending_review", stuck_since: "..."},
    {kind: "hu", id, status: "in_progress", stuck_since: "...", req_slug: "REQ-12"},
    {kind: "sync", id, status: "conflict", stuck_since: "...", external_key: "DIDE-200"},
    ...
  ]
Paginada y ordenable
```

### Escenario 12: Eventos al event bus

```gherkin
Dado que cada transition es auditada
Cuando ocurre una transition
Entonces se publica al event bus (REQ-10.3) evento "lifecycle.transition" con payload:
  {entity_kind, entity_id, from_state, to_state, actor, occurred_at, organization_id}
Y suscriptores externos pueden reaccionar (ej. enviar email al cliente cuando HU pasa a "done")
```

### Escenario 13: Compensation de transitions (rollback)

```gherkin
Dado que una transition tuvo side effects (ej. push a Jira)
Y se descubre que fue error y se quiere revertir
Cuando admin invoca `domain_lifecycle_compensate({transition_id, reason})`
Entonces:
  1. Se intenta deshacer side effect (driver Jira: PUT issue volviendo al estado anterior)
  2. Se crea NUEVA transition con `compensates_id=transition_id` (no se borra el original)
  3. Se audita
Y si el side effect no se puede revertir, se reporta y la transition queda visible como "compensada parcialmente"
```

### Escenario 14: Sabotaje — race condition en transition

```gherkin
Dado que 2 actores intentan transitionar el mismo HU simultáneamente (proposed→approved y proposed→rejected)
Cuando ambas requests llegan al servicio
Entonces se usa optimistic locking (column `version` en entidad + WHERE version=:expected)
Y solo una transition tiene éxito, la otra recibe 409 conflict con `current_state`, `current_version`
Y el actor losing puede reintentar con el estado actual si aplica
```

### Escenario 15: Permission scopes granulares

```gherkin
Dado que existen permisos:
  hu:transition:approve, hu:transition:reject, hu:transition:archive
Cuando se valida transition
Entonces se mapea (from_state, to_state) → permission requerido
Y se invoca check permission vía REQ-02.2
Y un user puede tener subset (ej. solo aprobar pero no rechazar)
```

## Esquema BD

```sql
CREATE TABLE state_machine_rules (
  id BIGSERIAL PRIMARY KEY,
  entity_kind VARCHAR(20) NOT NULL,
    -- intake | req | hu | sync_state
  from_state VARCHAR(30) NOT NULL,
  to_state VARCHAR(30) NOT NULL,
  required_permission VARCHAR(60),
    -- ej "hu:transition:approve", null = system-only
  conditions JSONB,
    -- expression evaluable, ej {"req.status": "active"}
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (entity_kind, from_state, to_state)
);
-- Seed inicial con rules para intake/req/hu/sync_state via issue-01.7

CREATE TABLE entity_state_transitions (
  id BIGSERIAL PRIMARY KEY,
  organization_id UUID NOT NULL,
  entity_kind VARCHAR(20) NOT NULL,
  entity_id UUID NOT NULL,
  from_state VARCHAR(30),
  to_state VARCHAR(30) NOT NULL,
  actor_kind VARCHAR(20) NOT NULL,
    -- user | agent | system | webhook | cron
  actor_id TEXT,
    -- user UUID, agent_id, "scheduler", external service name
  actor_display_name TEXT,
  reason TEXT,
  context JSONB,
    -- payload arbitrario (jira webhook id, intake id, etc.)
  source_event_id TEXT,
    -- correlation
  forced BOOLEAN NOT NULL DEFAULT false,
  compensates_id BIGINT REFERENCES entity_state_transitions(id),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  tx_id TEXT
);
CREATE INDEX ON entity_state_transitions (entity_kind, entity_id, occurred_at DESC);
CREATE INDEX ON entity_state_transitions (organization_id, occurred_at DESC);
-- Sin UPDATE/DELETE: enforced con RLS policy + revoke desde role app

CREATE TABLE entity_stuck_flags (
  entity_kind VARCHAR(20) NOT NULL,
  entity_id UUID NOT NULL,
  current_state VARCHAR(30) NOT NULL,
  stuck_since TIMESTAMPTZ NOT NULL,
  rule_violated TEXT NOT NULL,
    -- e.g. "intake.pending_review > 7d"
  notified_at TIMESTAMPTZ,
  resolved_at TIMESTAMPTZ,
  PRIMARY KEY (entity_kind, entity_id)
);

CREATE OR REPLACE VIEW lifecycle_overview AS
  SELECT
    r.id as req_id, r.slug as req_slug, r.status as req_status, r.created_at,
    COUNT(*) FILTER (WHERE hu.status = 'proposed') as hu_proposed,
    COUNT(*) FILTER (WHERE hu.status = 'approved') as hu_approved,
    COUNT(*) FILTER (WHERE hu.status = 'in_progress') as hu_in_progress,
    COUNT(*) FILTER (WHERE hu.status = 'done') as hu_done,
    COUNT(*) FILTER (WHERE hu.status = 'rejected') as hu_rejected,
    COUNT(*) FILTER (WHERE hu.status = 'archived') as hu_archived,
    COUNT(DISTINCT i.id) as intake_count,
    COUNT(DISTINCT s.id) FILTER (WHERE s.sync_status = 'conflict') as drift_count,
    MAX(t.occurred_at) as last_activity_at
  FROM requirements r
  LEFT JOIN issues hu ON hu.req_id = r.id
  LEFT JOIN intake_to_req_links itr ON itr.req_id = r.id
  LEFT JOIN intake_payloads i ON i.id = itr.intake_id
  LEFT JOIN external_sync_state s ON (s.entity_kind = 'req' AND s.entity_id = r.id) OR (s.entity_kind = 'hu' AND s.entity_id = hu.id)
  LEFT JOIN entity_state_transitions t ON (t.entity_kind = 'req' AND t.entity_id = r.id) OR (t.entity_kind = 'hu' AND t.entity_id = hu.id)
  GROUP BY r.id;
```

## MCP tools

| tool | input | output |
|------|-------|--------|
| `domain_lifecycle_overview` | `{organization_id, filter?}` | `{reqs[]}` |
| `domain_lifecycle_timeline` | `{entity_kind, entity_id}` | `{transitions[], enriched_events[]}` |
| `domain_lifecycle_trace` | `{start_kind, start_id}` | `{graph}` |
| `domain_lifecycle_query` | `{state? \| stuck?, since?, limit?}` | `{entities[]}` |
| `domain_lifecycle_metrics` | `{organization_id, period}` | `{throughput, lead_time_p50, p90, cycle_times, ...}` |
| `domain_lifecycle_force_transition` | `{entity_kind, entity_id, to_state, override_reason}` | `{transition_id}` |
| `domain_lifecycle_compensate` | `{transition_id, reason}` | `{compensation_id}` |
| `domain_lifecycle_transition` | `{entity_kind, entity_id, to_state, reason?, expected_version?}` | `{transition_id, new_state, new_version}` |

## Análisis breve

- **Qué pide:** capa de state machine + audit unificado + métricas lifecycle + stuck detector
- **Módulos sospechados:** `internal/sdd/lifecycle/{service, rules, metrics, stuck}.go` + view materialized opcional + cron job
- **Dependencias hard:** issue-04.1, issue-04.2, issue-04.8 (intake states), issue-04.9 (sync states), REQ-02.2 (RBAC), REQ-02.4 (audit)
- **Dependencias soft:** REQ-10.3 event bus (publish lifecycle.transition), REQ-20 (notifications stuck)
- **Riesgos:** view materialized requiere refresh strategy; race conditions en transition → optimistic lock; rules expr engine puede crecer en complejidad → keep simple en MVP (igualdad de campo, no DSL completo)
- **Esfuerzo tentativo:** L
