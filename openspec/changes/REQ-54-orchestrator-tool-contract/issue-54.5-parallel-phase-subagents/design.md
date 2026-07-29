# issue-54.5 parallel-phase-subagents — Design

## Decisions

- **D1. El plan es dato declarativo, no código.** `Output.SubagentPlan` (string
  markdown estructurado) en phases/registry.go — mismo circuito probado de
  RequiredToolCalls (54.1): handler default → PhaseStep → step.Inputs →
  rebuildOutputFromStepInputs → prompt. Override en
  `agent_templates.metadata.subagent_plan` (gana si presente), editable en
  prod sin recompilar.

- **D2. Inyección en el user_prompt, en LOS DOS puntos.** Igual que
  prepared_context (54.2): hydrateSystemPrompts (plan inicial) +
  rebuildNextStepPrompt (lazy rebuild de Full). Bloque con encabezado
  "## Plan de subagentes (ejecutar en paralelo)".

- **D3. Teeth vía el shape del Output, no vía confianza.** Donde el fan-out es
  crítico (judge), el `handler.Validate` exige el resultado del paralelismo:
  `judge_verdicts` con length >= 2 y campos por juez. Un solo juez no puede
  fabricar el shape sin mentir explícitamente — y el contrato 54.1 puede exigir
  además las tools que cada rol llama.

- **D4. N es sugerencia, el cliente adapta.** El plan declara roles y N
  sugerido; el cliente puede reducir por presupuesto. Lo NO negociable es el
  shape del Output (D3), no el número exacto de subagentes.

- **D5. Pilotos con planes concretos:**
  - `sdd-explore`: "faneá 1 Explore por área que detectes (máx 4): rutas,
    servicios, esquema, tests. Cada uno mapea su área; mergeá en un mapa
    único con referencias file:line."
  - `sdd-verify`: "agrupá los escenarios Gherkin en lotes independientes y
    validalos en subagentes paralelos; reportá scenarios_passed/failed por
    lote mergeado."
  - `sdd-judge`: "lanzá 2 jueces adversariales CIEGOS (sin ver el veredicto
    del otro): uno correctness, uno security/robustez. Output judge_verdicts
    con ambos; si divergen, tercer juez desempata." (= judgment-day).

## Alternatives

- **A1. Fan-out server-side con MiniMax (agent_run × N).** Determinista pero
  invierte el modelo económico (server paga) y MiniMax es más débil que el LLM
  del cliente justo donde importa (judge). Queda para fase 2 solo en fases
  read-only async.
- **A2. Skill del cliente (tipo judgment-day genérica) en vez de plan por
  fase.** Menos control por fase y no editable desde la BD del orquestador.
  El plan por fase puede REFERENCIAR skills existentes del cliente.

## Data Flow

```
handler.Build (sdd-judge)
  └─ Output{SubagentPlan: "## Plan de subagentes\n- juez A (correctness)...", ...}
       └─ PhaseStep.SubagentPlan → step.Inputs["subagent_plan"]
            └─ prompt de la fase = subagent_plan + prepared_context + user_prompt
cliente recibe el prompt
  └─ fanea 2 jueces ciegos en paralelo (sus subagentes)
  └─ mergea → phase_result{output: {judge_verdicts: [v1, v2]}, tool_calls: [...]}
       └─ server: handler.Validate exige len(judge_verdicts) >= 2  ← teeth
```

## TDD Plan

1. `TestSubagentPlan_FlowsToPromptInitial`: handler con plan → prompt del step
   del plan inicial contiene el bloque.
2. `TestSubagentPlan_FlowsToPromptLazyRebuild`: idem en rebuildNextStepPrompt.
3. `TestSubagentPlan_TemplateOverride_Wins`: metadata.subagent_plan pisa el
   default del handler.
4. `TestSubagentPlan_Empty_NoOp`: fase sin plan → prompt sin el bloque.
5. `TestJudgeValidate_RequiresMultipleVerdicts`: output con 1 verdict →
   rechazo; con 2 → pasa.

## Risk Mitigation

- Todo aditivo y retrocompatible: plan vacío = comportamiento actual.
- El bloque del plan va DESPUÉS del prepared_context para que el agente lea
  primero el contexto y después cómo distribuirlo.
