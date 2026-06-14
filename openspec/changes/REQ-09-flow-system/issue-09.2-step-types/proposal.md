# Proposal: issue-09.2-step-types

## Intención

Implementar los 10 tipos de paso como engines ejecutables independientes que comparten una interfaz común `StepRunner`. Cada tipo define su propio schema de parámetros, su lógica de ejecución y el formato de resultado. El flow runner (issue-09.3) orquesta los steps delegando en estos runners según el tipo.

## Scope

**Incluye:**
- Interfaz `StepRunner` con método `Run(ctx, step, flowContext) (StepResult, error)`
- Implementación de los 10 step types
- Validación de schema por tipo (campos requeridos obligatorios y opcionales)
- Resolución de templates (`{{variable}}`) en parámetros antes de ejecución
- Manejo de timeout por paso

**Excluye:**
- Sandboxing real de code_exec (se integra con REQ-11.1)
- La orquestación y state machine (issue-09.3)
- Retry policies (issue-09.4)
- Sub-flows composition semántica (issue-09.5)

## Enfoque técnico

- Interfaz `StepRunner` en `internal/flow/step_types/runner.go`:
  ```go
  type StepRunner interface {
      Type() string
      ValidateParams(params map[string]interface{}) error
      Run(ctx context.Context, step models.Step, flowCtx *FlowContext) (*StepResult, error)
  }
  ```
- Registry pattern: `map[string]StepRunner` populado en init() de cada tipo
- Cada runner en su propio archivo: `skill_call.go`, `llm_call.go`, `code_exec.go`, `conditional.go`, `parallel.go`, `wait.go`, `human_input.go`, `domain_agent_run.go`, `sub_flow.go`, `transform.go`
- Template resolution: `text/template` con funciones helper (default, upper, lower, json)
- Parallel runner: `errgroup` con contexto cancelable para concurrencia
- Wait runner: `time.After` o ticker para condición + `context.WithTimeout`

## Riesgos

- code_exec sin sandbox real es un riesgo de seguridad crítico → debe integrarse con sandbox container (REQ-11.1) o ejecutarse en modo seguro con restricciones estrictas (no net, no fs write)
- human_input requiere almacenamiento persistente de tareas pendientes y un mecanismo de callback/webhook para reanudar el flow
- conditional parsing: la condición es string arbitrario → evaluador seguro (expr libraries como `expr` o `cel-go`) en vez de `eval`

## Testing

- Unit: cada step type con parámetros válidos e inválidos
- Unit: template resolution edge cases (variables anidadas, defaults, escapes)
- Unit: parallel con N branches, una falla
- Unit: wait con duración y con condición
- Integration: skill_call contra skill registry real
- Integration: llm_call contra mock LLM provider
- Sabotaje: omitir campo requerido → test de validación falla
