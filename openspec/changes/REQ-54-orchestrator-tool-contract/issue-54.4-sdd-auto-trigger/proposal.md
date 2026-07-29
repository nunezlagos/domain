# Como agente cliente, cuando el usuario pide un requerimiento recibo una señal DETERMINISTA (inyectada por el hook UserPromptSubmit con la clasificación que devuelve el server) que me instruye a orquestar con domain_orchestrate antes de implementar, de modo que el pipeline SDD sea el camino default y no dependa de mi memoria.

## Why

Hoy NADA dispara el SDD: el pipeline arranca solo si el agente decide llamar
`domain_orchestrate`, y esa decisión vive en prosa (domain.md). Evidencia en
prod: sesiones enteras de features implementadas sin un solo orchestrate.

La pieza que lo hace determinista YA existe desde hoy: el hook
`UserPromptSubmit` captura cada prompt vía `domain_prompt_capture`, y los
hooks de Claude Code pueden emitir `hookSpecificOutput.additionalContext`
(contexto inyectado que el agente no puede ignorar). Falta que el server
devuelva la clasificación y que el hook la convierta en señal.

## Scope

1. **Server**: `domain_prompt_capture` responde además `classification`
   {intent, complexity, suggested_action} reusando `analyzeComplexity` del
   orquestador (heurístico léxico, cero LLM, cero latencia extra).
2. **Hook**: `domain-user-prompt.sh` emite additionalContext cuando
   `suggested_action=orchestrate`: "este prompt parece un requerimiento
   (complexity=X); ejecutá domain_orchestrate antes de implementar; si hay
   un flow activo, retomalo".
3. **Policy**: platform policy `sdd-auto-trigger` en BD como norma formal
   (feature→orchestrate, bug→ticket, trivial→directo).
4. **First-response**: mostrar `active_flow_run` para retomar en vez de
   re-orquestar.
5. **Métrica**: ratio orchestrate-calls / prompts-clasificados-feature en
   `mcp_tool_invocations` + `prompt_captured`.

Fuera de alcance: forzar server-side el inicio del flow (imposible: el server
no controla al cliente); clasificación con LLM (el heurístico alcanza para v1).

## Approach

Reusar TODO: analyzeComplexity (orquestador), el hook de captura (recién
instalado), el formato additionalContext (mismo del SessionStart). El único
código nuevo real es el campo en la respuesta del capture y ~15 líneas de
bash en el hook.

## Risks

- Falsos positivos del heurístico molestan ("orquestá" en prompts triviales)
  → el additionalContext es sugerencia fuerte, no bloqueo; umbral: solo
  complexity moderate/complex sugieren orchestrate.
- Prompts con flow ya activo → la señal debe decir "retomá el flow X", no
  "creá otro" (el server sabe si hay active_flow_run del proyecto).

## Testing

Unit del clasificador expuesto en capture; test del hook con stdin fake
verificando el JSON de additionalContext; métrica consultable.
