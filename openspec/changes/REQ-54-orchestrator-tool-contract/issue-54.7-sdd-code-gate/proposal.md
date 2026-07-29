# Como plataforma domain, ninguna edición de código ocurre sin flow SDD activo: en modo normal el humano decide en el diálogo de permisos, en modos automáticos el agente es forzado a orquestar — y toda duda se consulta al usuario antes de redactar el spec.

## Why
Gap observado en prod: el agente recibió la señal 54.4 y implementó directo sin
orquestar (flow_runs vacío). Decisión del usuario: TODO código pasa por SDD,
sin exención trivial.

## Scope
1. Gate determinista (hooks Claude Code): PostToolUse sobre domain_orchestrate/
   flow_status/phase_result/confirm marca flow-activo por sesión; PreToolUse
   sobre Edit|Write|NotebookEdit|Bash-heurística intercepta sin flow: ask
   (default/plan) / deny (modos automáticos).
2. Policy sdd-auto-trigger v2 imperativa (BD, version 2).
3. Fase sdd-spec instruye CONSULTAR dudas (AskUserQuestion) antes de redactar.
4. Bonus worktrees: hooks resuelven project_slug vía git-common-dir (un
   worktree ya no atribuye capturas a un proyecto fantasma).

Limitación documentada: heurística Bash (sed -i, tee, patch, git apply,
redirect a extensión de código) con falsos negativos posibles — auditable.
