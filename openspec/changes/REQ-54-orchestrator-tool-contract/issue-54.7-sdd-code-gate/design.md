# issue-54.7 sdd-code-gate — Design

## Decisions
- D1. Estado por sesión en ~/.local/state/domain/: flow-<session_id> (marca de
  flow activo, la escribe PostToolUse). El gate NO depende de la clasificación
  del turno: TODO código se gatea — la clasificación solo sugiere el mode.
- D2. ask vs deny por permission_mode del stdin de PreToolUse: default/plan →
  ask (humano decide); resto (acceptEdits/bypassPermissions/auto) → deny.
- D3. Bash: heurística de edición. Comandos inocuos no gatean (verificado).
- D4. Escape hatch: solo el usuario aprueba ediciones sin SDD.
- D5. Worktrees: slug por git rev-parse --git-common-dir, no basename(cwd).

## Validación en producción (dogfooding, 2026-07-02)
El gate bloqueó al propio agente implementador (deny en modo auto, falso
positivo por contenido de heredoc — mecanismo correcto); el agente orquestó
(flow e73c7079, lite), PostToolUse marcó la sesión y el gate abrió. El
contrato de explore rechazó el primer phase_result sin tool_calls
(missing_tool_calls=[domain_code_graph]) y aceptó el reintento con el
contrato satisfecho. Hallazgo: los hooks HOT-RELOAD (aplican a la sesión
corriendo, sin reinicio).

## TDD (6 escenarios simulados por stdin — verdes)
ask default / deny acceptEdits / Bash inocuo pasa / sed -i gatea /
flow marker abre / post-orchestrate marca.
