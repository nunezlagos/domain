# Proposal: issue-09.3-flow-state-machine

## Intención

Implementar una state machine determinística que gobierne la ejecución de flows, desde `pending` hasta `completed`/`failed`/`paused`/`cancelled`, con transiciones event-driven. Cada step tiene su propio sub-estado. El contexto de ejecución (resultados de steps anteriores) se pasa entre steps. Se expone API REST para consultar estado, pausar, reanudar y cancelar ejecuciones.

## Scope

**Incluye:**
- Modelo `FlowRun` con estado global + mapa de estados por step
- State machine con transiciones válidas (diagrama de estados)
- Flow runner que orquesta steps: lee el DAG, determina próximos steps según dependencias completadas
- API REST: POST /flows/:slug/run, GET /flow-runs/:id, POST /flow-runs/:id/pause, POST /flow-runs/:id/resume, POST /flow-runs/:id/cancel
- Context object compartido entre steps (lectura/escritura de resultados)
- Event bus interno: step_completed, step_failed → state machine reacciona
- Streaming de estado vía SSE (Server-Sent Events) para UI

**Excluye:**
- Retry policies (issue-09.4)
- Sub-flow execution (issue-09.5)
- Step type implementations (issue-09.2) — solo los invoca

## Enfoque técnico

- State machine como struct con métodos: `Transition(event)`, `AllowedTransitions()`, `CurrentState()`
- Flow runner como worker que:
  1. Construye el DAG desde steps del flow
  2. Calcula dependencias con topological order
  3. Procesa steps: cuando un step completa, revisa qué steps tienen dependencias satisfechas y los encola
  4. Ejecuta steps concurrentemente según el DAG (parallel branches)
- Context: `map[string]StepResult` donde key es step_id, se pasa a cada runner
- Persistencia: `FlowRun` en DB con steps JSONB de estado
- Pause/Resume: context cancel + snapshot de estado en DB; resume recrea el contexto
- SSE: endpoint GET /flow-runs/:id/stream que emite eventos de estado

## Riesgos

- Estado inconsistente si el runner crashea a mitad de un step → usar WAL (Write-Ahead Log) de eventos de estado antes de ejecutar cada step
- Parallel steps: race condition al actualizar el contexto compartido → usar mutex por step ID (no contexto global)
- Pause en medio de un step de human_input: el step debe soportar cancelación limpia (context.Context)

## Testing

- Unit: state machine transitions — todas las combinaciones válidas e inválidas
- Unit: DAG traversal — determinar próximos steps según estado actual
- Unit: context passing entre steps
- Integration: ejecución completa de flow contra DB
- Integration: pause/resume cycle
- Integration: cancel en medio de ejecución
- E2E: POST run → GET status → esperar completion
- Sabotaje: transición inválida (completed→running) debe retornar error
