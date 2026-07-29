# issue-54.2 server-side context prep — Design

## Decisions

- **D1. La preparación es un hook por fase, no un modo nuevo.** Antes de que el orquestador
  entregue el prompt de una fase, corre `prepareContext(phase, flowRun)` que agrega un
  bloque `prepared_context` a `PriorOutputs`. No cambia el modelo client-driven: solo
  enriquece lo que el cliente recibe.

- **D2. Dos niveles de preparación: crudo e inteligente.**
  - *Crudo* (siempre): correr las tools read-only pertinentes a la fase (ej. `policy_list`,
    `project_skill_list`, `mem_context`, `code_graph`). Puro acceso a BD, sin LLM.
  - *Inteligente* (si Minimax disponible): pasar el crudo por Minimax para filtrar/resumir
    ("de estas 19 policies, estas 3 aplican a sdd-apply"). Con timeout corto.

- **D3. Degradación elegante.** Si Minimax no está (key ausente) o excede el timeout, se
  entrega el contexto crudo sin filtrar. La fase nunca se bloquea por la preparación.

- **D4. Mapeo fase→tools-de-preparación, declarativo.** Cada fase declara qué tools
  read-only alimentan su contexto (`PrepToolCalls() []string`), separado de las
  `required_tool_calls` del cliente (54.1). Ejemplo: `sdd-apply` prep =
  `[policy_list, project_skill_list]`; `sdd-explore` prep = `[code_graph, mem_context]`.

- **D5. Reuso del motor DAG donde encaje.** Para preparaciones multi-tool, expresar el
  prep como un sub-flow del `flowrunner` (que ya ejecuta server-side con step types
  `skill_call`/`parallel`). Evita un ejecutor paralelo nuevo. Donde sea una sola tool,
  llamada directa.

## Alternatives

- **A1. Que el cliente llame las tools de prep (todo client-side).** Es el fallback de
  54.1 (contrato estricto). Más simple pero mantiene la fricción y la latencia del lado del
  cliente. La prep server-side es superior cuando Minimax puede filtrar.
- **A2. Preparación en background antes de que el cliente pida la fase.** Ideal pero
  requiere predecir la próxima fase; se posterga. Fase 1 = prep on-demand al entrar a la
  fase.

## Data Flow

```
cliente pide siguiente step (fase sdd-apply)
        │
        ▼  server: prepareContext("sdd-apply", flowRun)
   crudo   = run(policy_list) + run(project_skill_list)          (BD, sin LLM)
   filtrado= minimax.summarize(crudo, "cuáles aplican a apply")  (si disponible, timeout 3s)
   prepared_context = filtrado ?? crudo                          (degradación)
        │
        ▼  prompt de la fase += prepared_context
   cliente recibe el prompt CON policies/skills ya seleccionadas
   cliente ejecuta la fase con su LLM (trabajo creativo)
```

## TDD Plan

1. `TestPrepContext_Raw_RunsReadOnlyTools`: fase con prep `[policy_list]` → `prepared_context`
   contiene las policies de BD.
2. `TestPrepContext_Minimax_Filters`: con Minimax mock → `prepared_context` es el resumen
   filtrado, no el crudo.
3. `TestPrepContext_NoMinimax_DegradesToRaw`: sin key Minimax → `prepared_context` = crudo,
   sin error.
4. `TestPrepContext_Timeout_DegradesToRaw`: Minimax excede timeout → crudo, fase no bloqueada.
5. `TestPrepContext_PerPhaseMapping`: cada fase corre solo sus `PrepToolCalls`.

## Risk Mitigation

- Timeout duro en la llamada a Minimax; fallback a crudo.
- La prep es aditiva: si falla entera, la fase corre como hoy (sin `prepared_context`).
- Depende de issue-54.3 para la key de Minimax en el proceso Go; hasta entonces, corre en
  modo crudo (útil igual).
