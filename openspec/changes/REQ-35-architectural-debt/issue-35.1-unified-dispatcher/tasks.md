# Tasks: issue-35.1-unified-dispatcher

## Backend

- [ ] **T1]: Crear `internal/dispatch/dispatcher.go`:
  - Struct `Dispatcher` con deps (Pool, FlowRunner, AgentRunner,
    SkillRunner, Audit, Metrics, Logger).
  - Struct `Request` con todos los campos.
  - Struct `Result`.
  - `Dispatch(ctx, req) (Result, error)` con el switch
    centralizado.
  - Audit pre y post con source + target_type.
  - Métricas pre y post con labels (source, target_type, result).

- [ ] **T2]: Crear `internal/dispatch/flow.go` con `runFlow(ctx,
  req) (Result, error)`:
  - Llama `flowRunner.Start(ctx, req.TargetID, req.Inputs,
    req.OrgID, ...)`.
  - Retorna `Result{RunID: flowRunID, Status: "started"}`.

- [ ] **T3]: Crear `internal/dispatch/agent.go` con `runAgent(ctx,
  req) (Result, error)`:
  - Llama `agentRunner.Start(ctx, req.TargetID, req.Inputs, ...)`.
  - Retorna `Result{RunID: agentRunID, Status: "started"}`.

- [ ] **T4]: Crear `internal/dispatch/skill.go` con `runSkill(ctx,
  req) (Result, error)`:
  - Llama `skillRunner.Execute(ctx, req.TargetID, req.Inputs, ...)`.
  - Retorna `Result{RunID: skillExecutionID, Status: "started"}`.

- [ ] **T5]: Crear `internal/dispatch/metrics.go`:
  - `metrics.DispatchTotal *prometheus.CounterVec` con labels
    source, target_type, result.
  - `metrics.DispatchDuration *prometheus.HistogramVec` con
    labels source, target_type.
  - Registrar en `internal/metrics/registry.go`.

- [ ] **T6]: Phase 2: migrar CRON.
  - Modificar `internal/scheduler/cron/scheduler.go` para usar
    `dispatcher.Dispatch` en vez del switch local.
  - Tests de paridad: pre-refactor (switch local) vs
    post-refactor (dispatcher) deben dar el mismo flow_run_id
    para los mismos inputs.

- [ ] **T7]: Phase 3: migrar WEBHOOK.
  - Modificar `internal/service/webhook/` para delegar al
    dispatcher.
  - Tests de paridad.

- [ ] **T8]: Phase 4: migrar MCP.
  - Modificar `cmd/domain-mcp/` para que las 3 tools
    (`domain_flow_run`, `domain_agent_run`,
    `domain_skill_execute`) usen el dispatcher.
  - Alternativa: unificar en 1 tool
    `domain_dispatch(target_type, target_id, inputs)`.
  - Tests de paridad.

- [ ] **T9]: Phase 5: eliminar código viejo.
  - `cronsched.dispatchSync` → ELIMINAR.
  - `webhook.dispatchWebhook` → ELIMINAR.
  - `mcp.handleFlowRun`, `handleAgentRun`, `handleSkillExecute`
    → ELIMINAR (o unificarlos).
  - Test: `TestOldDispatchersRemoved` con grep que verifica
    que ninguna función vieja existe.

- [ ] **T10]: Dual-write de métricas (1 release):
  - Mantener los 3 counters viejos (`CronDispatchedTotal`,
    `WebhookDispatchedTotal`, `MCPDispatchedTotal`) Y el nuevo
    `DispatchTotal` con labels.
  - Después de 1 release con Prometheus queries validadas →
    eliminar los viejos.

- [ ] **T11]: Wire en `cmd/domain/main.go`:
  - Instanciar el dispatcher una vez.
  - Pasarlo al cron scheduler, al webhook service, y al MCP
    client.

## Tests

- [ ] `TestDispatcher_FlowRuns**` — Dispatch con
  TargetType=flow → llama al flow runner con los inputs.
- [ ] `TestDispatcher_AgentRuns**` — Dispatch con TargetType=agent
  → llama al agent runner.
- [ ] `TestDispatcher_SkillExecutes**` — Dispatch con
  TargetType=skill → llama al skill runner.
- [ ] `TestDispatcher_UnknownTypeFails**` — TargetType="unknown" →
  error "unknown target_type", métricas con result=failed, audit
  con error.
- [ ] `TestDispatcher_AuditLogsStartedAndCompleted**` — Dispatch
  completo → 2 entries en audit_log (started + completed).
- [ ] `TestDispatcher_MetricsRecorded**` — Dispatch → 1 increment
  en DispatchTotal + 1 observation en DispatchDuration.
- [ ] `TestDispatcher_ParityWithOldCron**` — mismo flow + inputs
  ejecutados por el cron viejo y el dispatcher nuevo →
  mismo flow_run_id, misma duración (within tolerance), mismas
  métricas.
- [ ] `TestOldDispatchersRemoved**` — grep el código, assserta
  que ninguna de las funciones viejas existe.
- [ ] `T-sabotaje`: Comentar la migración del WEBHOOK (phase 3)
  → test e2e que assserta "los 3 sources ejecutan un nuevo
  target_type" DEBE FALLAR (webhook no usa dispatcher) →
  restaurar migración → test verde. Documentar en commit body.
