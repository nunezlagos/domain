# Domain MCP — protocolo de uso

Este sistema tiene **Domain MCP** registrado (server: `domain`, tools con
prefijo `domain_*`). Domain centraliza memoria persistente, clasificación
de prompts, orquestación SDD, skills, agents, flows y catálogo de
proyectos/clientes. **Es el punto de entrada por defecto** antes de
responder o actuar.

---

## Regla de oro (cada turno)

Antes de responderle al usuario o tocar archivos:

1. **Clasificá** el mensaje llamando `domain_orchestrate` con `raw_text=<mensaje>`
   y `project_slug=<slug-del-proyecto-actual>` si lo conocés.
   - El response trae `intent` (`chat | idea | feature | fix | hotfix | refactor | doc | rfc | analysis`) y, para intents accionables, un `plan` con steps (system_prompt + user_prompt + suggested_saves).
   - Si `intent=chat | idea` → el reply trae instrucciones inline; seguí el protocolo de ese reply (mem_search → responder → mem_save).
   - Si `intent=feature | fix | refactor | ...` → ejecutá los steps del plan en orden, reportando cada uno con `domain_orchestrate_phase_result`.

2. **Recuperá contexto** con `domain_mem_search` y/o `domain_mem_context`
   antes de generar la respuesta. No inventes historia que no leíste.

3. **Trabajá** (Edit/Bash/Read son tuyos — Domain no toca código).

4. **Persistí** lo no-obvio con `domain_mem_save` antes de cerrar el turno.

---

## Tools por caso de uso

| Caso | Tool(s) Domain |
|---|---|
| Clasificar mensaje + arrancar pipeline | `domain_orchestrate`, `domain_orchestrate_phase_result`, `domain_orchestrate_confirm` |
| Memoria persistente (decisiones, bugs, gotchas, prefs) | `domain_mem_save`, `domain_mem_search`, `domain_mem_context`, `domain_mem_get_observation`, `domain_mem_delete` |
| Sesiones + timeline | `domain_session_start`, `domain_session_end`, `domain_session_active`, `domain_timeline` |
| Catálogo de proyectos | `domain_project_list`, `domain_project_create`, `domain_project_update` |
| Catálogo de clientes/mandantes | `domain_client_list`, `domain_client_get`, `domain_client_create`, `domain_client_update`, `domain_client_delete`, `domain_client_restore`, `domain_client_set_status` |
| Knowledge base (SOPs, runbooks, ADRs) | `domain_knowledge_save`, `domain_knowledge_search`, `domain_knowledge_get` |
| Skills versionados (prompts + validación) | `domain_skill_list`, `domain_skill_search`, `domain_skill_get`, `domain_skill_execute` |
| Agents (multi-step) | `domain_agent_list`, `domain_agent_get`, `domain_agent_create`, `domain_agent_run`, `domain_agent_run_logs` |
| Flows (DAGs) | `domain_flow_list`, `domain_flow_run`, `domain_flow_status`, `domain_flow_create` |
| HU wizard (creación guiada de historias de usuario) | `domain_hu_create_start`, `domain_hu_create_answer`, `domain_hu_create_preview`, `domain_hu_create_commit`, `domain_hu_create_abandon`, `domain_hu_drafts_list` |
| Intake de ideas/feedback | `domain_intake_submit`, `domain_intake_list_pending`, `domain_intake_approve`, `domain_intake_reject`, `domain_intake_get` |
| Policies & guardrails | `domain_policy_list`, `domain_policy_get` |
| Prompts reutilizables | `domain_prompt`, `domain_prompt_get`, `domain_prompt_search`, `domain_prompt_render`, `domain_mem_save_prompt` |
| Búsqueda global (atajo) | `domain_search_global` |

> Si dudás entre 2 tools, elegí la más específica. Si una tool `domain_*`
> aplica → preferila sobre alternativas locales (TODO files, scratchpads,
> notes en disco).

---

## Patrón mem_save

Siempre estructurá el `content`:

```
**What**: qué se hizo (concreto, una frase)
**Why**: motivación (problema/usuario/contexto)
**Where**: paths o módulos afectados
**Learned**: gotchas, edge cases, decisiones (omitir si nada)
```

Tipos válidos: `decision | architecture | bugfix | pattern | config | discovery | learning`.

---

## Subagents, worktrees y loops

Si el cliente IDE expone estas primitivas (Claude Code 2026+, OpenCode), úsalas
en combinación con Domain:

- **Subagents** → tareas paralelas independientes (review + test + lint en
  simultáneo). Cada subagent debería al inicio hacer `domain_mem_search` de su scope y al final reportar a Domain con `domain_mem_save`.
- **Worktrees** (`--worktree` en Claude Code, `isolation: worktree` en
  Workflow) → cuando varios subagents van a editar el mismo repo en paralelo, cada uno en su checkout aislado.
- **/loop** (Claude Code skill bundled) → para chequeos recurrentes
  (p.ej. monitor CI, polling de un PR). Si el loop debe sobrevivir a
  sesiones, persistir el estado en `domain_mem_save` cada iteración.
- **/goal** → loop con condición de parada verificable. Mapea 1:1 al
  `domain_orchestrate` con phases SDD: cada phase tiene validación D5
  que define cuándo está "done". Si la phase falla validación, el flow
  se queda en `requires_confirm` hasta que el usuario apruebe.

---

## Convenciones

- Idioma del cliente IDE manda (español si el usuario habla español).
- Tools MCP siempre con prefijo `domain_`.
- Nunca uses `~/notes/`, `TODO.md`, scratchpads o archivos sueltos para
  estado que sobrevive al turno. Eso es `domain_mem_save`.
- Nunca leas/escribas `.env` para secrets que Domain pueda servir.
