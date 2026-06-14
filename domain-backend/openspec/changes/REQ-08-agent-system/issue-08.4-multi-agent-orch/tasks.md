# Tasks: issue-08.4-multi-agent-orch

## Backend

- [x] Implementar `AgentOrchestrator` con métodos `Delegate` y `ParallelRun`
- [x] Implementar skill especial `delegate_to_agent` para exponer en tool registry del supervisor
- [x] Implementar sub-run creation: cada delegación crea sub-run vinculado al run padre
- [x] Implementar handoff protocol: serializar contexto como JSON con metadata (_delegated_from, _parent_run_id)
- [x] Implementar ejecución paralela: goroutines + waitgroup + result channel
- [x] Implementar detección de ciclos y max_depth (default 3)
- [x] Implementar max_concurrent (default 5) con semáforo
- [x] Implementar timeout por subagente (configurable, default 120s)
- [x] Manejar errores de subagente: capturar como resultado, no como excepción
- [x] Exponer endpoint POST /agents/:id/run con parámetro `parallel_agents` opcional

## Tests

- [x] Test unitario: delegación simple supervisor → subagente
- [x] Test unitario: handoff context pasa datos correctamente
- [x] Test unitario: ejecución paralela (3 tareas concurrentes)
- [x] Test unitario: max_depth detecta ciclo A→B→A
- [x] Test unitario: max_concurrent respeta límite
- [x] Test unitario: timeout de subagente retorna error al supervisor
- [x] Test unitario: error en subagente se captura como resultado
- [x] Test de integración: orquestador + agentes reales
- [x] Test E2E: escenarios Gherkin del hu.md
- [x] Sabotaje: ciclo de delegación → max_depth retorna error

## Cierre

- [x] Verificación manual: flujo supervisor + 2 subagentes reales
- [x] Suite verde completa
- [x] Documentar protocolo de handoff y configuración de orquestación
