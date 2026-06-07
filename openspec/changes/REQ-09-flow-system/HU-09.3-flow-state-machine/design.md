# Design: HU-09.3-flow-state-machine

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| State machine | Struct con métodos (Go simple) | Stateless library, workflow engine externo (temporal.io) — sobreingeniería para MVP |
| Event bus | In-process channel (`chan Event`) | RabbitMQ/NATS (innecesario para mono-servicio, channel es suficiente) |
| Concurrencia en DAG | Goroutines por step + errgroup | BFS secuencial (descartado: no soporta parallel naturalmente) |
| Persistencia de estado | Updates atómicos a DB por transición | WAL separado (se puede agregar después, por ahora DB es suficiente) |
| Streaming | SSE (Server-Sent Events) | WebSocket (SSE es más simple para stream unidireccional) |

## Alternativas descartadas

- **Temporal.io / Cadence**: Potente pero es otro sistema externo. Para MVP, una state machine en proceso es suficiente. Si en futuro se necesita durabilidad跨-máquina, se migra.
- **Graph-based workflow engine (Dagre)**: Solo UI, no lógica de negocio.
- **Event sourcing completo**: Demasiado overhead para MVP. Guardar snapshots de estado es suficiente.

## Diagrama

```
State Machine - FlowRun:
┌──────────┐
│  pending │
└────┬─────┘
     │ start
     ▼
┌──────────┐
│ running  │◄────────────┐
└──┬───┬───┘             │
   │   │                 │
   │   │ step failed     │
   │   ▼                 │
   │ ┌────────┐          │
   │ │ failed │          │
   │ └────────┘          │
   │                     │
   │ pause               │
   ▼                     │
┌──────────┐  resume     │
│  paused  ├─────────────┘
└──┬───────┘
   │ cancel
   ▼
┌───────────┐
│ cancelled │
└───────────┘

   │ last step completed
   ▼
┌───────────┐
│ completed │
└───────────┘

State Machine - StepRun:
┌──────────┐
│ pending  │
└────┬─────┘
     │ dependencies met
     ▼
┌───────────┐
│ step_run. │
│ -ning     │◄──────────┐
└───┬───┬───┘           │
    │   │               │
    │   │ error         │
    │   ▼               │
    │ ┌─────────┐       │
    │ │step_fail│       │
    │ │ -ed     │       │
    │ └─────────┘       │
    │                   │
    │ pause             │
    ▼                   │
┌──────────┐  resume    │
│ step_pau.│────────────┘
│ -sed     │
└───┬──────┘
    │ cancel
    ▼
┌──────────┐
│ step_can.│
│ -celled  │
└──────────┘

    │ success
    ▼
┌───────────┐
│ step_comp.│
│ -leted    │
└───────────┘
```

Flujo interno del runner:
```
1. Create FlowRun (status: pending)
2. Start → status: running
3. Build dependency graph de steps
4. Encontrar steps con in-degree 0 (sin dependencias)
5. Para cada step listo:
   a. Set step status: step_running
   b. Launch goroutine con StepRunner.Run()
   c. Esperar resultado
   d. On success: set step_completed, store result en context
   e. On error: set step_failed, flow → failed (si no hay manejo de error)
   f. Recalcular próximos steps disponibles
6. Si no hay más steps → flow → completed
7. Si pause → cancel goroutines activas → snapshot → paused
8. Si cancel → cancel goroutines activas → cancelled
```

## TDD plan

1. **Red:** Test `TestStateMachine_Transitions` — todas las transiciones válidas
2. **Green:** Implementar FlowStateMachine con mapa de transiciones
3. **Red:** Test `TestStateMachine_InvalidTransition` — error en transición inválida
4. **Green:** Rechazar transiciones no permitidas
5. **Red:** Test `TestFlowRunner_LinearDAG` — secuencia s1→s2→s3
6. **Green:** Implementar runner con BFS sobre DAG
7. **Red:** Test `TestFlowRunner_StepFailure` — flow falla en s2
8. **Green:** Propagar error y detener ejecución
9. **Red:** Test `TestFlowRunner_PauseResume` — pausar y reanudar
10. **Green:** Context cancel + snapshot + restore
11. **Red:** Test `TestFlowRunner_Cancel` — cancelar en medio
12. **Green:** Cancel context y marcar como cancelled
13. **Red:** Test `TestContextPassing` — step hijo ve resultado de step padre
14. **Green:** FlowContext con map compartido
15. **Sabotaje:** Remover verificación de transición → test de transición inválida falla

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Crash durante ejecución de step | Media | Alto | WAL: persiste estado ANTES de ejecutar cada step, no después |
| Deadlock en DAG con parallel | Baja | Alto | Timeout global en flow run + context.WithTimeout |
| State machine inconsistente | Baja | Medio | Tests de tabla con todas las combinaciones de transiciones |
| SSE conexión perdida | Alta | Bajo | Reconnect del lado cliente, último estado en GET |
