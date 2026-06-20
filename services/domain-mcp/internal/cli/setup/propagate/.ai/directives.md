# Directivas para agentes AI — proyecto Domain

Este proyecto tiene **Domain MCP** registrado. Usá las tools `domain_*` para:

- **Memoria persistente**: `domain_mem_save` para guardar observaciones,
  `domain_mem_search` para recuperar contexto previo.
- **Agents**: `domain_agent_run` para ejecutar agentes definidos
  en la plataforma con tools + skills.
- **Flows**: `domain_flow_run` para orquestaciones multi-step.
- **Skills**: `domain_skill_execute` para invocar skills nativos
  (prompts versionados, API calls validadas).
- **Prompts**: `domain_prompt_render` con templates parametrizados.

**Antes de actuar**: chequeá memoria con `domain_mem_search` para evitar
trabajo duplicado y mantener continuidad cross-sesión.

**Al terminar tareas no triviales**: guardá un resumen con
`domain_mem_save` (decisión, bug fix, convention, gotcha) — esto
preserva contexto para futuras sesiones del mismo proyecto.

## Configs sensibles

NUNCA modificar archivos:
- `.env`, `.env.*` (secrets locales)
- `.git/` (estado del repo)
- `*.pem`, `*.key` (private keys)
- `credentials.*`, `*credentials*`
- `*.kube/config` (cluster creds)

Si una tarea requiere modificar uno de esos → preguntá explícito al humano antes.
