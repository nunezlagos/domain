# Design: issue-09.2-step-types

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Evaluación de condiciones | `expr` (github.com/expr-lang/expr) | `eval`, `cel-go` (expr es seguro, type-safe, sin reflection insegura) |
| Template engine | `text/template` estándar | `quicktemplate`, `handlebars` (text/template es suficiente y no requiere dep externa) |
| Concurrencia parallel | `errgroup` de `golang.org/x/sync` | sync.WaitGroup (errgroup propaga primer error automáticamente) |
| Registry de runners | `map[string]StepRunner` con init() | interface registration explícita (init() es simple para MVP) |
| code_exec sandbox | Delegar a REQ-11.1 | Runner propio con gvisor/nsjail (fuera de scope, REQ-11.1 lo cubre) |

## Alternativas descartadas

- **CEL (Common Expression Language)**: Similar a expr, pero expr tiene mejor DX en Go y sintaxis más familiar.
- **Ejecución secuencial de parallel**: Descartado — parallel sin concurrencia real no tiene sentido.
- **Hardcode de tipos en switch**: Descartado — registry pattern permite extensión sin modificar el core.

## Diagrama

```
┌─────────────────────────────────────────────────────────────┐
│                    StepRunner interface                      │
├─────────────────────────────────────────────────────────────┤
│ Type() string                                                │
│ ValidateParams(params) error                                 │
│ Run(ctx, step, flowCtx) (*StepResult, error)                 │
└─────────────────────────────────────────────────────────────┘
                              ▲
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│ SkillCallRunner│    │  LLMCallRunner│    │ CodeExecRunner│
├───────────────┤    ├───────────────┤    ├───────────────┤
│ skill_slug     │    │ prompt_templ. │    │ script        │
│ input_mapping  │    │ model         │    │ language      │
└───────────────┘    │ temperature   │    │ sandbox_mode  │
                     └───────────────┘    └───────────────┘
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│ ConditionalRun.│    │ ParallelRunner│    │  WaitRunner   │
├───────────────┤    ├───────────────┤    ├───────────────┤
│ condition(expr)│   │ branches[]    │    │ duration_secs │
│ if_branch[]    │    │ max_concurr. │    │ until_cond.   │
│ else_branch[]  │    └───────────────┘    │ poll_interval │
└───────────────┘                          └───────────────┘
┌───────────────┐    ┌───────────────┐    ┌───────────────┐
│HumanInputRun. │    │ AgentRunRunner│    │  SubFlowRunner│
├───────────────┤    ├───────────────┤    ├───────────────┤
│ question       │    │ agent_slug    │    │ flow_slug     │
│ timeout_hours  │    │ input_mapping │    │ input_mapping │
│ assignee       │    └───────────────┘    └───────────────┘
└───────────────┘
┌───────────────┐
│TransformRunner│
├───────────────┤
│ expression    │
│ engine(jq/jp) │
└───────────────┘
```

Flujo de ejecución de un step:
1. FlowRunner obtiene step de la cola
2. Resuelve templates en params con contexto actual
3. Busca runner en registry por `step.Type`
4. Valida params con runner.ValidateParams()
5. Ejecuta runner.Run() con context timeout
6. Almacena resultado en FlowContext.Steps[step.ID]
7. Emite evento step_completed → state machine avanza

## TDD plan

1. **Red:** Test `TestSkillCallRunner_Valid` espera resultado exitoso
2. **Green:** Implementar SkillCallRunner con mock del skill engine
3. **Red:** Test `TestLLMCallRunner_Template` espera prompt resuelto
4. **Green:** Implementar LLMCallRunner con template resolution
5. **Red:** Test `TestCodeExecRunner_Script` espera resultado del script
6. **Green:** Implementar CodeExecRunner (sandbox mode stub)
7. **Red:** Test `TestConditionalRunner_IfBranch` ejecuta if_branch
8. **Green:** Implementar ConditionalRunner con expr
9. **Red:** Test `TestParallelRunner_Concurrent` ejecuta N branches
10. **Green:** Implementar ParallelRunner con errgroup
11. **Red:** Test `TestWaitRunner_Duration` espera N segundos
12. **Green:** Implementar WaitRunner con time.After
13. **Red:** Test `TestHumanInputRunner_CreateTask` crea tarea pendiente
14. **Green:** Implementar HumanInputRunner con store de tareas
15. **Red:** Test `TestAgentRunRunner_Delegation` lanza agente
16. **Green:** Implementar AgentRunRunner
17. **Red:** Test `TestTransformRunner_JsonPath` transforma datos
18. **Green:** Implementar TransformRunner con jsonpath lib
19. **Sabotaje:** Quitar validación de skill_slug → test falla

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Código inseguro en code_exec | Alta | Crítico | Sandbox obligatorio (REQ-11.1), modo "safe" por defecto, modo "unsafe" requiere flag explícito |
| Human input perdido | Media | Alto | Store persistente de tareas + background worker que reintenta notificar cada 1h |
| Template injection | Media | Alto | text/template escapa por defecto; limitar funciones disponibles |
| Conditional expr lento | Baja | Medio | Timeout de 5s en evaluación de expresión |
