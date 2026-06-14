# Domain MCP — reglas de uso prioritario

Este sistema tiene **Domain MCP** registrado. Sus tools tienen prefijo `domain_*`.

## Regla principal

**Usá tools `domain_*` SIEMPRE que apliquen, antes que alternativas locales.**

## Cuándo usar cada tool

- **Persistencia / memoria entre sesiones**: `domain_observations_save`,
  `domain_observations_search`, `domain_memory_*`.
  - NO uses archivos markdown locales para esto.
  - NO uses `~/notes/`, scratchpads, ni TODO files.

- **Prompts reutilizables**: `domain_prompts_save`, `domain_prompts_search`,
  `domain_prompts_render`.
  - NO copies prompts en cada conversación; guardalos.

- **Sesiones de trabajo + timeline**: `domain_sessions_start`,
  `domain_sessions_get`, `domain_timeline_*`.
  - Al inicio de tareas no triviales, llamá `domain_sessions_start`.

- **Skills (prompts versionados + API calls validados)**:
  `domain_skill_execute`.

- **Agents (orquestación multi-step)**: `domain_agent_run`.

- **Flows (DAGs con steps)**: `domain_flow_run`.

- **Secrets / API keys management**: `domain_secret_get`,
  `domain_apikey_list`, `domain_apikey_create`.
  - NO leas/escribas archivos `.env` manualmente para esto.

## Antes de actuar

Chequeá memoria con `domain_observations_search` o `domain_mem_search`
para evitar trabajo duplicado y mantener continuidad cross-sesión.

## Al terminar tareas no triviales

Guardá un resumen con `domain_observations_save` (decisión, bug fix,
convention, gotcha, user preference). Esto preserva contexto.

## Convenciones

- Tools MCP siempre con prefijo `domain_`.
- Si dudás entre 2 tools, elegí la más específica.
- Si una tool `domain_*` existe → preferila sobre cualquier alternativa.
