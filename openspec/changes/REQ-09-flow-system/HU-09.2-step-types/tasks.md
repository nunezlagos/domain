# Tasks: HU-09.2-step-types

## Backend

- [ ] Definir interfaz `StepRunner` en `internal/flow/step_types/runner.go`
- [ ] Crear registry de runners (`map[string]StepRunner`) con registro automático
- [ ] Implementar template resolver (`text/template` con funciones helper)
- [ ] Implementar `SkillCallRunner` — invoca skill por slug
- [ ] Implementar `LLMCallRunner` — prompt template → LLM provider
- [ ] Implementar `CodeExecRunner` — ejecuta script sandboxeado (stub hasta REQ-11.1)
- [ ] Implementar `ConditionalRunner` — evaluador expr, if/else branches
- [ ] Implementar `ParallelRunner` — errgroup con concurrencia controlada
- [ ] Implementar `WaitRunner` — time.After y condición evaluada por polling
- [ ] Implementar `HumanInputRunner` — crea tarea, callback para reanudar
- [ ] Implementar `AgentRunRunner` — delega ejecución a sistema de agentes
- [ ] Implementar `SubFlowRunner` — lanza sub-flow y espera resultado
- [ ] Implementar `TransformRunner` — JSONPath y jq (gojq)
- [ ] Agregar validación de schema por tipo en crear/actualizar flow
- [ ] Agregar resolución de templates antes de ejecutar cada step
- [ ] Agregar timeout por paso (default 300s, configurable)

## Tests

- [ ] Test unitario: SkillCallRunner válido e inválido (sin slug)
- [ ] Test unitario: LLMCallRunner con template resuelto correctamente
- [ ] Test unitario: LLMCallRunner con modelo y temperatura específicos
- [ ] Test unitario: CodeExecRunner script simple
- [ ] Test unitario: CodeExecRunner script con error
- [ ] Test unitario: ConditionalRunner rama if
- [ ] Test unitario: ConditionalRunner rama else
- [ ] Test unitario: ConditionalRunner condición inválida
- [ ] Test unitario: ParallelRunner 3 branches exitosos
- [ ] Test unitario: ParallelRunner con una branch fallida
- [ ] Test unitario: WaitRunner duración exacta
- [ ] Test unitario: WaitRunner condición
- [ ] Test unitario: WaitRunner timeout
- [ ] Test unitario: HumanInputRunner crea tarea pendiente
- [ ] Test unitario: HumanInputRunner callback reanuda
- [ ] Test unitario: HumanInputRunner timeout
- [ ] Test unitario: AgentRunRunner delega
- [ ] Test unitario: SubFlowRunner referencia circular (detección)
- [ ] Test unitario: TransformRunner JSONPath
- [ ] Test unitario: TransformRunner jq
- [ ] Test de integración: secuencia de tipos mixtos
- [ ] Sabotaje: quitar validación de skill_slug → test falla

## Cierre

- [ ] Verificación manual: crear flow con todos los tipos de step
- [ ] Suite verde
