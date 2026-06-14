# Design: issue-35.1-unified-dispatcher

## Contexto

El código de domain tiene 3 lugares donde se decide "qué
ejecutar" dado un target:

1. **Cron** (`internal/scheduler/cron/`): lee steps del cron
   (`run_flow`, `run_agent`, `run_skill`) y los dispatcha.
2. **Webhook** (`internal/service/webhook/`): recibe un POST
   externo, parsea el payload, dispatcha.
3. **MCP** (`cmd/domain-mcp/`): recibe una tool call
   (`domain_flow_run`, `domain_agent_run`, `domain_skill_execute`),
   dispatcha.

Cada uno tiene su propio switch con su propia lógica, métricas,
audit, manejo de errores. El día que se agrega un nuevo
TargetType ("workflow_v2", "pipeline", etc.) hay que tocar 3
lugares. Bug clásico: se olvida uno y ese source queda roto.

## Decisión arquitectónica

**Estrategia:** interface `Dispatcher` con UN switch
centralizado + 3 call-sites que delegan.

1. **Interface `internal/dispatch/Dispatcher`:**
   ```go
   package dispatch

   type Request struct {
     OrgID        uuid.UUID
     Source       string  // "cron" | "webhook" | "mcp" | "manual"
     TargetType   string  // "flow" | "agent" | "skill"
     TargetID     uuid.UUID
     Inputs       json.RawMessage
     TriggeredBy  *uuid.UUID
     Traceparent  string  // para distributed tracing
   }

   type Result struct {
     RunID  uuid.UUID  // flow_run_id, agent_run_id, o skill_execution_id
     Status string     // "started" | "completed" | "failed"
     Output json.RawMessage
   }

   type Dispatcher struct {
     Pool       *pgxpool.Pool
     FlowRunner *flowrunner.Runner
     AgentRunner *agentrunner.Runner
     SkillRunner *skillrunner.Runner  // global
     Audit      *audit.PGRecorder
     Metrics    *metrics.Registry
     Logger     *slog.Logger
   }

   func (d *Dispatcher) Dispatch(ctx, req Request) (Result, error) {
     timer := prometheus.NewTimer(metrics.DispatchDuration.WithLabelValues(req.Source, req.TargetType))
     defer timer.ObserveDuration()

     // Audit: started
     d.audit.Record(ctx, audit.Event{
       OriginOrgID: &req.OrgID,
       Actor: actorFromTriggeredBy(req.TriggeredBy),
       Action: "dispatch.started",
       Resource: fmt.Sprintf("%s/%s", req.TargetType, req.TargetID),
       Metadata: map[string]any{"source": req.Source},
     })

     var result Result
     var err error
     switch req.TargetType {
     case "flow":
       result, err = d.runFlow(ctx, req)
     case "agent":
       result, err = d.runAgent(ctx, req)
     case "skill":
       result, err = d.runSkill(ctx, req)
     default:
       err = fmt.Errorf("unknown target_type: %s", req.TargetType)
     }

     // Metrics + audit: result
     resultLabel := "success"
     if err != nil { resultLabel = "failed" }
     metrics.DispatchTotal.WithLabelValues(req.Source, req.TargetType, resultLabel).Inc()
     d.audit.Record(ctx, audit.Event{
       OriginOrgID: &req.OrgID,
       Action: "dispatch.completed",
       Resource: fmt.Sprintf("%s/%s", req.TargetType, req.TargetID),
       Metadata: map[string]any{"source": req.Source, "result": resultLabel, "error": errMsg(err)},
     })

     return result, err
   }
   ```

2. **Migración de los 3 call-sites:**
   - **Cron** (`internal/scheduler/cron/scheduler.go`): el
     switch actual de `step.Type → runner` se reemplaza por:
     ```go
     result, err := dispatcher.Dispatch(ctx, dispatch.Request{
       OrgID: cron.OrgID,
       Source: "cron",
       TargetType: step.Type,
       TargetID: step.TargetID,
       Inputs: step.Inputs,
     })
     ```
   - **Webhook** (`internal/service/webhook/`): el handler
     que parsea el payload llama al dispatcher con
     `Source: "webhook"`.
   - **MCP** (`cmd/domain-mcp/`): las 3 tools
     (`domain_flow_run`, `domain_agent_run`,
     `domain_skill_execute`) se unifican en 1 tool genérica
     `domain_dispatch(target_type, target_id, inputs)` que
     llama al dispatcher. O se mantienen 3 tools pero
     internamente todas llaman al dispatcher.

3. **Eliminación de código viejo:**
   - Las funciones `dispatchSync`, `dispatchWebhook`,
     `handleFlowRun` (MCP), `handleAgentRun`, `handleSkillExecute`
     se ELIMINAN (no quedan como dead code wrappers).
   - Test `TestOldDispatchersRemoved`: grep el código fuente,
     assserta que ninguna de esas funciones existe.

4. **Backward compat:** el comportamiento observable NO cambia.
   El flow corre igual, los errores son los mismos, las
   métricas son las mismas (más una nueva unificada).
   Los timeouts del issue 33.3 siguen aplicando (el dispatcher
   pasa el context con el budget).

5. **Migración gradual (low risk):**
   - Phase 1: crear el dispatcher + metrics + audit unificados.
   - Phase 2: migrar CRON (más simple, menos call-sites).
   - Phase 3: migrar WEBHOOK.
   - Phase 4: migrar MCP.
   - Phase 5: eliminar código viejo.
   - Cada phase tiene su PR + tests de paridad
     (comportamiento idéntico al pre-refactor).

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Mantener los 3 dispatchers pero agregar tests de consistencia | Los tests pueden romperse y nadie los lee. Centralizar es más robusto. |
| B | Strategy pattern (cada target_type es un handler registrado) | Más complejo (registro dinámico). El switch es simple y suficiente. |
| C | Message queue (cron/webhook/mcp publican a una cola, 1 worker dispatcha) | Overkill para el caso. Agrega infra. |
| D | Plugin system (3rd party puede agregar dispatchers) | Out of scope. Solo los 3 internos. |

## Por qué interface + switch centralizado gana

- **DRY:** la lógica vive en 1 lugar. Agregar target_type = 1
  case nuevo.
- **Observable:** métricas y audit unificados permiten queries
  cross-source triviales.
- **Backward compat:** el refactor es NO-trivially-behavior-changing.
  El mismo flow corre igual.
- **Testeable:** el dispatcher se testea unitariamente. Los
  call-sites se testean con mocks del dispatcher.

## Detalle de implementación

- `internal/dispatch/dispatcher.go` con la interface.
- `internal/dispatch/flow.go`, `agent.go`, `skill.go` con los
  métodos privados `runFlow`, `runAgent`, `runSkill`.
- `internal/dispatch/metrics.go` con `DispatchTotal` y
  `DispatchDuration` (ya existentes o nuevos).
- Migration en 4 phases (ver "Migración gradual" arriba).
- Tests: paridad (pre-refactor vs post-refactor, mismo input
  → mismo output).

## Riesgos

- **R1:** Refactor gigante rompe algo. **Mitigación:** migration
  en 4 phases con tests de paridad por phase. Cada phase es
  deployable independientemente.
- **R2:** El switch crece mucho con muchos target_types.
  **Aceptable:** hasta 10 types es manejable. Si pasa, refactor
  a un registry.
- **R3:** Métricas cambian de nombre → alerting en Prometheus
  se rompe. **Mitigación:** mantener los 3 counters viejos
  también durante 1 release (dual-write), después eliminar.
