# issue-54.3 async worker + fixes — Design

## Decisions

- **D1. Worker async = tailer de flow_runs pendientes en modo async.** Un worker de fondo
  (goroutine lanzada al arranque del server, con `context` cancelable) hace polling de
  `flow_runs` con `status=pending AND exec_mode/mode=async`, toma uno con lock, y llama
  `ProcessAsyncFlowRun`. Alternativa considerada: goroutine disparada directamente por
  `runAsync` — más simple pero se pierde si el proceso reinicia. El tailer es durable
  (relee de BD). Se elige el tailer.

- **D2. Requiere LLM.** `ProcessAsyncFlowRun` necesita `s.LLM != nil`. Por eso el fix #2
  (key en env) es prerequisito del worker. Si no hay LLM, el worker no arranca y loguea el
  motivo (no falla el server).

- **D3. Opt-in y acotado.** El worker solo procesa flows creados explícitamente async.
  Límite de concurrencia configurable. Nunca toca flows interactivos (client-driven).

- **D4. Fix de compose es aditivo y usa las vars canónicas.** Agregar al bloque
  `environment` de `domain-mcp`: `LLM_PROVIDER`, `LLM_API_KEY`, `LLM_MODEL` (las canónicas
  de `.env.example:144-172`), con los aliases `MINIMAX_*` como respaldo. Referenciar desde
  `services/.env` vía `${VAR}`.

- **D5. Wiring de binarios: unificar la construcción del orquestador.** Extraer el armado
  del orquestador (con Spec/Tasks/IssueSvc) a una función compartida usada por ambos
  `cmd/domain` y `cmd/domain-mcp`, para que `persistOpenspec` no sea no-op en ninguno. Si
  `cmd/domain-mcp` es legacy, documentar su deprecación en su lugar.

- **D6. Código muerto: eliminar por defecto.** `agent/orchestration/*` sin call-sites →
  eliminar salvo que haya un plan concreto de uso. Renombrar/documentar la relación
  `agents` (ejecución real, `domain_agent_run`) vs `agent_templates` (solo system_prompts
  de fases) para quitar la confusión.

## Alternatives

- **Cron en vez de worker goroutine** (usar `domain_cron_*`): viable, pero acopla el
  lifecycle del async a la infra de cron. La goroutine tailer es autocontenida.

## Data Flow (worker async)

```
server arranca
  └─ if LLM != nil: go asyncWorker(ctx)
        loop cada N seg:
          run = repo.ClaimNextPendingAsyncFlow()   (lock)
          if run == nil: continue
          ProcessAsyncFlowRun(ctx, run)             (ejecuta steps con Minimax)
             └─ por cada step: provider.Complete → Validate → MarkStepCompleted
          emite flow_signals; marca flow completed/failed
```

## TDD Plan

1. `TestAsyncWorker_ProcessesPendingFlow`: flow async pending → worker lo lleva a
   `completed`; no queda `pending`.
2. `TestAsyncWorker_NoLLM_DoesNotStart`: sin LLM → worker no arranca, server sano.
3. `TestAsyncWorker_IgnoresInteractiveFlows`: flow no-async → worker no lo toca.
4. `TestAsyncWorker_Concurrency`: respeta el límite configurado.
5. `TestPersistOpenspec_BothBinaries` (o test de wiring): el orquestador construido por la
   función compartida tiene Spec/Tasks/IssueSvc no-nil.
6. Compose: test/health que confirme que el proceso ve `LLM_API_KEY` (o smoke manual
   documentado).

## Risk Mitigation

- Worker con `context` cancelable y límite de concurrencia → no se desboca.
- Fix de compose no cambia comportamiento si la var no está (degradación a sin-LLM).
- Eliminar código muerto va en su propio commit, reversible.
