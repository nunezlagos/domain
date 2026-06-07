# Tasks: HU-08.4-multi-agent-orch

## Backend

- [ ] Implementar `AgentOrchestrator` con métodos `Delegate` y `ParallelRun`
- [ ] Implementar skill especial `delegate_to_agent` para exponer en tool registry del supervisor
- [ ] Implementar sub-run creation: cada delegación crea sub-run vinculado al run padre
- [ ] Implementar handoff protocol: serializar contexto como JSON con metadata (_delegated_from, _parent_run_id)
- [ ] Implementar ejecución paralela: goroutines + waitgroup + result channel
- [ ] Implementar detección de ciclos y max_depth (default 3)
- [ ] Implementar max_concurrent (default 5) con semáforo
- [ ] Implementar timeout por subagente (configurable, default 120s)
- [ ] Manejar errores de subagente: capturar como resultado, no como excepción
- [ ] Exponer endpoint POST /agents/:id/run con parámetro `parallel_agents` opcional

## Tests

- [ ] Test unitario: delegación simple supervisor → subagente
- [ ] Test unitario: handoff context pasa datos correctamente
- [ ] Test unitario: ejecución paralela (3 tareas concurrentes)
- [ ] Test unitario: max_depth detecta ciclo A→B→A
- [ ] Test unitario: max_concurrent respeta límite
- [ ] Test unitario: timeout de subagente retorna error al supervisor
- [ ] Test unitario: error en subagente se captura como resultado
- [ ] Test de integración: orquestador + agentes reales
- [ ] Test E2E: escenarios Gherkin del hu.md
- [ ] Sabotaje: ciclo de delegación → max_depth retorna error

## Cierre

- [ ] Verificación manual: flujo supervisor + 2 subagentes reales
- [ ] Suite verde completa
- [ ] Documentar protocolo de handoff y configuración de orquestación
