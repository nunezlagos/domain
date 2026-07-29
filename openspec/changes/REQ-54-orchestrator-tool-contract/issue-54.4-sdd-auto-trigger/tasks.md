# issue-54.4 sdd-auto-trigger — Tasks

## 1. Server: clasificación en el capture
- [ ] Verificar si captured_prompt puede importar analyzeComplexity sin ciclo;
      si hay ciclo, extraer a paquete leaf (internal/service/complexity) y
      redirigir el import del orquestador.
- [ ] `domain_prompt_capture` responde `classification: {complexity,
      suggested_action, suggested_mode}` (aditivo, retrocompatible).
- [ ] Lookup de `active_flow_run` del proyecto (estado no-terminal) →
      `active_flow_run_id` + fase actual en la respuesta.
- [ ] Tests 1-3 del TDD plan.

## 2. Hook: emitir la señal
- [ ] `domain-user-prompt.sh`: parsear `classification` de la respuesta;
      si `suggested_action=orchestrate|resume`, emitir
      `hookSpecificOutput.additionalContext` por stdout (JSON).
- [ ] Textos: orchestrate → "ejecutá domain_orchestrate antes de implementar";
      resume → "retomá el flow <id> con domain_orchestrate_status, NUNCA
      re-orquestes".
- [ ] Test bash con stdin fake (complex → señal; trivial → sin señal).

## 3. Policy + first-response
- [ ] Platform policy `sdd-auto-trigger` (confirmación humana síncrona antes
      de persistir, per lifecycle de policies).
- [ ] first-response: incluir `active_flow_run` si existe.

## 4. Métrica
- [ ] Query/documentación: ratio orchestrate-calls vs prompts complex en
      mcp_tool_invocations + prompt_captured (para medir adopción real).

## 5. Distribución
- [ ] Los cambios del hook llegan por install-curl.sh (ya instala los scripts);
      redeploy VPS para el cambio del server.
