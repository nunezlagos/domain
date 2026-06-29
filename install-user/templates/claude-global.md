# Domain — precedencia global (SIEMPRE ACTIVO)

Este usuario usa **domain** como plataforma de memoria persistente, policies
SDD, skills, agents y flows. **domain tiene prioridad** sobre cualquier otro
sistema de memoria conectado, y esto aplica en CUALQUIER repositorio que abras
(es global, no por proyecto). Si una regla local contradice a domain en esos
temas, **seguí domain**. Las reglas TÉCNICAS del repo (estilo de código, stack,
workflow, comandos) **siguen valiendo** — domain las IMPORTA a BD con
`domain_project_policy_import_from_text` para que queden versionadas. El
archivo del usuario queda intacto; el import solo crea una copia versionada.

## Cadena de tools (resumen ejecutivo — el detalle está en la policy viva)

Domain expone ~145 tools `domain_*` que se encadenan en 5 paths críticos.
El protocolo COMPLETO y vivo se carga con:

    domain_policy_get(slug="agent-protocol")

Editar esa policy actualiza el comportamiento de TODOS los agentes sin
tocar archivos. Es la fuente canónica — este bloque es solo el mapa.

### Path A — Sesión (auto, en cada turn)
```
[Turn 1]   domain_prompt_capture(content)              # UNA vez por turn, raw_text usuario
           domain_session_bootstrap(cwd, git_remote, git_branch, git_head,
                                    existing_rules_files)
           ├── known=true  → leer recent_observations + counts
           │   └── si head.changed → mem_save lo relevante del diff
           └── known=false → domain_session_register + project_index (REQ-62)
[Durante]  domain_mem_save(type, topic_key, body)      # CADA decisión/fix/patrón
           domain_knowledge_save                       # docs/chunks con embeddings
           domain_mem_search / domain_search_global    # cuando pida "acordate"
           domain_mem_context(project_slug)            # recuperar contexto
[Cierre]   domain_turn_complete                         # fin de turn (passive capture)
           domain_session_summary                      # al cerrar sesión
```

### Path B — SDD (10 fases — ver grafo en admin /flujo-sdd/)
```
sdd-explore → sdd-spec → sdd-propose → sdd-design → sdd-tasks →
sdd-apply → sdd-verify → sdd-judge → sdd-archive → sdd-onboard
```
- Issue formal (Gherkin): `domain_issue_create_start` → `_answer` (loop) →
  `_preview` → `_commit`. Alias legacy: `domain_hu_create_*`.
- Verifications (TDD adversarial): `domain_verify_start` →
  `domain_verify_update_item` → `domain_verify_complete`.
- Proposals diferidas: `domain_propose_skill` / `domain_propose_policy` SOLO
  en modo headless/batch. Con usuario presente → confirmar y crear activa.

### Path C — Tickets (operativo, no SDD)
- `domain_ticket_create` para bugs/tasks/features sin Gherkin. Status
  workflow kanban. Mover con `domain_ticket_change_status` (no update directo).
- Si el ticket implementa una issue formal, vincular con
  `domain_ticket_link_issue(ticket_id, issue_id)`.

### Path D — Stack skills (project-scoped, monorepo-aware)
1. Detectar TODOS los roots de stack (package.json, go.mod, composer.json,
   .gitmodules, etc). Monorepo = N skills, no 1.
2. Por cada stack nuevo: armar skill ("framework-major-stack",
   prefijando subpath si no es root) → confirmar con usuario →
   `domain_project_skill_register` con `root_path`.
3. Drift de stack: si bootstrap devuelve head.changed=true, verificar si
   los manifiestos cambiaron entre last_known_head y current → proponer
   `domain_skill_edit`.

### Path E — Orquestación / ejecuciones
- `domain_flow_create` / `domain_flow_run` / `domain_flow_status`.
- `domain_orchestrate` → `domain_orchestrate_phase_result` → `domain_orchestrate_confirm`.
- `domain_agent_run` / `domain_agent_run_logs`.
- Subagentes del cliente: el server NO tiene LLM. Fan-out con tu Task tool
  (Claude Code) o subagents (OpenCode) para paralelizar fases SDD.

## Re-hidratación tras compactación

Domain es PULL, no push. Si el contexto se compacta:
1. `domain_session_bootstrap` (recupera project + work_summary).
2. `domain_mem_context` para las últimas observaciones.
3. Mini-resumen al usuario: "venís trabajando en X, hay N tickets / M issues
   abiertos" + status de active_flow_run si lo hay.
4. Si `active_flow_run != null` → `domain_orchestrate_status` y RETOMÁ.
   NO reinicies; solo cambiá estado si el usuario ordena suspender.

## Skills/policies — confirmar ANTES de persistir

Toda skill o policy nueva, Y toda edición de una activa, pasa por
confirmación humana SÍNCRONA antes de escribirse. Sin importar origen
(detectada, pedida, inferida). NO persistas a ciegas.

1. Armá contenido completo.
2. Inferí SCOPE: interna (default, project_id=<proyecto>) vs global
   (project_id NULL, solo si verdad universal).
3. Mostrale al usuario contenido + scope → confirmación explícita.
4. Si modifica → re-mostrar con cambios aplicados. Repetir hasta confirmar.
5. Solo al confirmar, persistir: `domain_project_skill_register` /
   `domain_project_policy_set` (interna) o `domain_skill_create` /
   `domain_platform_policy_create` (global).
6. Dejar traza con `domain_mem_save` de qué se aprobó y por qué.

## Auto-persistencia — comportamiento por defecto

En TODA sesión de trabajo — sin importar el proyecto — domain-mcp persiste
automáticamente la actividad relevante. No esperes a que el usuario te lo pida.

### Al iniciar (después del bootstrap)
1. Si bootstrap devuelve `head.changed` o `recent_observations` no leídas →
   `domain_mem_context` + `domain_mem_search` antes de actuar.
2. Si hay policies SDD activas para el proyecto →
   `domain_policy_get(slug, project_slug)`.
3. Si `project_skill_count=0` → detectar stacks y proponer skill
   (vía Path D, con confirmación).

### Durante el trabajo
- Cada descubrimiento, decisión, patrón, fix, convención →
  `domain_mem_save(type=decision|fix|pattern|context|artifact, topic_key)`.
- Si el usuario corrige algo importante → `domain_mem_save(type=decision)`.
- Comandos significativos (deploy, migration, test suite) →
  `domain_mem_save` con el resultado.
- Una vez por turn → `domain_prompt_capture(content, project_slug?)` con
  el raw_text del usuario.

### Al cerrar turn / sesión
- `domain_turn_complete` al final de cada turn (señal de cierre).
- `domain_session_summary(accomplished, next_steps)` al cerrar sesión.
- Si hay tasks pendientes detectadas → `domain_ticket_create` o
  observations.

### Excepciones (cuándo NO persistir)
- Comandos triviales (ls, cat, git status sin cambios).
- Conversación pura sin aprendizaje técnico.
- Outputs efímeros (logs de runtime que ya están en otra BD).
- Outputs de queries de solo lectura (knowledge_get, mem_search).

Regla de oro: si te generó un "aha" técnico, persistilo. Si fue ruido, omitilo.

## Si un tool domain_* falla

"Connection closed" o key inválida → indicale al usuario correr /domain-login
o el installer. NO cambies a otro sistema de memoria como fallback silencioso.
Si un tool persiste con error 3+ veces, persistí el incidente con
`domain_mem_save(type=fix, topic_key="infra/domain-mcp/<código>")` y avisale
al usuario.

## Grafo SDD visual

El domain-admin expone el grafo completo de las 10 fases con tools y DB ops
por fase en `/flujo-sdd/` (renderizado desde `services/domain-admin/app/
templates/sdd_flow.html`). Usalo como referencia cuando planifiques una HU.
