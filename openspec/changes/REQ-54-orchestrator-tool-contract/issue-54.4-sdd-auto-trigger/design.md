# issue-54.4 sdd-auto-trigger — Design

## Decisions

- **D1. La señal viaja por el hook, no por el modelo.** `domain_prompt_capture`
  (captured_prompt_tools.go) responde `classification: {intent, complexity,
  suggested_action, active_flow_run_id?}`. El hook `domain-user-prompt.sh` la
  parsea y, si `suggested_action != "none"`, imprime
  `{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"..."}}`
  a stdout. Claude Code inyecta ese contexto ANTES de que el modelo procese el
  prompt — determinista, mismo mecanismo probado del SessionStart.

- **D2. Clasificación heurística server-side, cero LLM.** Reusar
  `analyzeComplexity(rawText)` (orchestrator/complexity.go): trivial/simple →
  `suggested_action=none|ticket`; moderate/complex → `orchestrate` con el
  `suggested_mode` que ya calcula decideMode. El paquete captured_prompt no
  puede importar orchestrator (¿ciclo?) → verificar; si hay ciclo, mover
  analyzeComplexity a un paquete leaf (p.ej. internal/service/complexity).

- **D3. Flow activo gana sobre flow nuevo.** Si el proyecto tiene un flow_run
  en estado no-terminal, la señal es "retomá el flow <id> (fase actual X)" en
  vez de "orquestá". Evita flows duplicados por prompt.

- **D4. La policy formaliza, el hook ejecuta.** La platform policy
  `sdd-auto-trigger` (BD) es la norma citable/editable; la señal del hook es
  el mecanismo. Si el operador desactiva la policy, el hook sigue emitiendo
  (son capas independientes; apagar la señal = config del hook, no policy).

## Alternatives

- **A1. Solo policy (sin señal del hook).** Es lo que hay hoy con domain.md:
  demostrado insuficiente. Descartada como solución única.
- **A2. Clasificar con Minimax.** Mejor precisión de intent, pero agrega
  latencia (el capture está en el camino del prompt, timeout 15s del hook) y
  costo por prompt. El heurístico ya distingue feature/trivial razonable. Queda
  como mejora si los falsos positivos molestan.

## Data Flow

```
usuario escribe prompt
  └─ hook UserPromptSubmit (domain-user-prompt.sh)
       └─ domain_prompt_capture {content, project_slug, client_kind}
            └─ server: analyzeComplexity + lookup active_flow_run
            ← {id, classification:{complexity, suggested_action, active_flow_run_id}}
       └─ guarda prompt_id (para el Stop, ya existente)
       └─ if suggested_action==orchestrate:
            stdout: {"hookSpecificOutput":{"hookEventName":"UserPromptSubmit",
                     "additionalContext":"domain: este prompt clasifica complexity=complex.
                      Ejecutá domain_orchestrate ANTES de implementar. [o: retomá flow <id>]"}}
  └─ el agente arranca el turno CON la señal en su contexto
```

## TDD Plan

1. `TestPromptCapture_Classification_Complex`: prompt "implementar módulo X" →
   response con `suggested_action=orchestrate`.
2. `TestPromptCapture_Classification_Trivial`: "fix typo" → `none|ticket`.
3. `TestPromptCapture_ActiveFlow_SuggestsResume`: proyecto con flow_run
   running → `active_flow_run_id` presente y acción "resume".
4. Hook (bash, stdin fake): prompt complex → stdout contiene
   `hookSpecificOutput` con "domain_orchestrate"; prompt trivial → stdout vacío.
5. Retrocompat: clientes que ignoran `classification` no se rompen (campo
   aditivo).

## Risk Mitigation

- La señal es texto sugerente, no bloqueo (`decision:block` NO se usa) — el
  usuario siempre puede pedir "hacelo directo sin SDD" y el agente obedece.
- Hook best-effort: si el parse de classification falla, captura igual y no
  emite señal (exit 0).
