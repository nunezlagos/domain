# Tasks: issue-09.2-step-types

## Backend

- [x] Definir interfaz `StepRunner` → `internal/runner/flow/steptypes/steptypes.go` (RunInput/StepRunner)
- [x] Crear registry de runners (`map[string]StepRunner`) con registro automático → `steptypes.go` NewRegistry/Register/Get
- [x] Implementar template resolver → `steptypes/template.go` (ResolveTemplate + ResolveBarePaths)
- [x] Implementar `SkillCallRunner` — invoca skill por slug → `steptypes/skill_call.go`
- [x] Implementar `LLMCallRunner` — prompt template → LLM provider → `steptypes/llm_call.go`
- [x] Implementar `CodeExecRunner` — stub hasta REQ-11.1 → `steptypes/code_exec.go`
- [x] Implementar `ConditionalRunner` — evaluador expr, if/else branches → `steptypes/conditional.go`
- [x] Implementar `ParallelRunner` — concurrencia controlada (32 branches max) → `steptypes/parallel.go`
- [x] Implementar `WaitRunner` — duración y condición por polling → `steptypes/wait.go`
- [x] Implementar `HumanInputRunner` — crea tarea pendiente → `steptypes/human_input.go`
- [x] Implementar `AgentRunRunner` — delega a sistema de agentes → `steptypes/agent_run.go`
- [x] Implementar `SubFlowRunner` — lanza sub-flow y espera resultado → `steptypes/sub_flow.go`
- [x] Implementar `TransformRunner` — JSONPath y jq → `steptypes/transform.go`
- [x] Agregar validación de schema por tipo en crear/actualizar flow → `service/flow/service.go` Spec.Validate
- [x] Agregar resolución de templates antes de ejecutar cada step → en cada runner via ResolveTemplate
- [x] Agregar timeout por paso (default 300s, configurable) → `runner/flow/runner.go` step.TimeoutS

## Tests

- [x] Test unitario: SkillCallRunner válido e inválido → TestSkillCallRunner_{Valid,MissingSlug,NoCaller,CallerError}
- [x] Test unitario: LLMCallRunner con template resuelto → TestLLMCallRunner_ValidWithTemplate
- [x] Test unitario: LLMCallRunner con modelo y temperatura → TestLLMCallRunner_WithTemperatureAndMaxTokens
- [x] Test unitario: CodeExecRunner script simple → TestCodeExecRunner_Stub (stub REQ-11.1)
- [x] Test unitario: CodeExecRunner script con error → TestCodeExecRunner_MissingScript (stub REQ-11.1)
- [x] Test unitario: ConditionalRunner rama if → TestConditionalRunner_IfBranch
- [x] Test unitario: ConditionalRunner rama else → TestConditionalRunner_ElseBranch
- [x] Test unitario: ConditionalRunner condición inválida → TestConditionalRunner_InvalidExpression
- [x] Test unitario: ParallelRunner 3 branches exitosos → TestParallelRunner_ThreeBranches
- [x] Test unitario: ParallelRunner con una branch fallida → TestParallelRunner_OneBranchFails
- [x] Test unitario: WaitRunner duración exacta → TestWaitRunner_Duration
- [x] Test unitario: WaitRunner condición → TestWaitRunner_ConditionMet
- [x] Test unitario: WaitRunner timeout → TestWaitRunner_DurationWithContextTimeout + ContextCancellation
- [x] Test unitario: HumanInputRunner crea tarea pendiente → TestHumanInputRunner_CreatesTask
- [x] Test unitario: HumanInputRunner callback reanuda → cubierto vía signals (issue-09.8 TestExpectSignal/WaitForSignal); el resume es responsabilidad del engine
- [x] Test unitario: HumanInputRunner timeout → cubierto por timeout de step (runner.go) + TestWaitRunner_ContextCancellation
- [x] Test unitario: AgentRunRunner delega → TestAgentRunRunner_{Valid,TemplateInInput}
- [x] Test unitario: SubFlowRunner referencia circular → TestSubflowCircular_DetectaCadenaRepetida
- [x] Test unitario: TransformRunner JSONPath → TestTransformRunner_{JSONPath,SimpleJSONPath,JSONPathArrayIndex,JSONPathWildcard}
- [x] Test unitario: TransformRunner jq → TestTransformRunner_JQ
- [x] Test de integración: secuencia de tipos mixtos → TestFlow_BasicSkillRun + TestFlow_MemSaveStep + TestFlow_OnErrorContinue (runner_integration_test.go)
- [x] Sabotaje: quitar validación de skill_slug → TestSkillCallRunner_Sabotage_NoValidation

## Cierre

- [x] Verificación manual: crear flow con todos los tipos de step → cubierto por suite integración (runner_integration_test.go) + registry round-trip (TestRegistryRoundTrip)
- [x] Suite verde → 2026-06-10: `go test -short ./...` 937 passed; `-tags=integration ./internal/runner/flow/... ./internal/service/flow/...` 269 passed
