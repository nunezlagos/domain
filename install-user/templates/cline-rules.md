# Domain MCP — Cline rules

> Este archivo se planta en `~/.clinerules-domain`. Cline lo lee automáticamente.

## Regla de oro (cada turno)

1. **Clasificá** el mensaje con `domain_orchestrate raw_text=<mensaje>` (+ `project_slug` si lo conocés).
   - `intent=chat | idea` → seguí el protocolo inline del reply.
   - `intent=feature | fix | refactor | hotfix | doc | rfc | analysis` → ejecutá los steps del plan en orden; reportá cada uno con `domain_orchestrate_phase_result`.
2. **Contexto** antes de responder: `domain_mem_search` (+ `domain_mem_context` si la conversación es continuación).
3. **Trabajá**.
4. **Persistí** lo no-obvio con `domain_mem_save` antes de cerrar.

## Tools clave

- Memoria: `domain_mem_save`, `domain_mem_search`, `domain_mem_context`, `domain_mem_delete`
- Orquestación: `domain_orchestrate`, `domain_orchestrate_phase_result`, `domain_orchestrate_confirm`
- Sesiones / timeline: `domain_session_start/end/active`, `domain_timeline`
- Catálogo: `domain_project_list/create/update`, `domain_client_list/get/create/update/delete/restore/set_status`
- Knowledge: `domain_knowledge_save/search/get`
- Skills / agents / flows: `domain_skill_execute`, `domain_agent_run`, `domain_flow_run`
- HU wizard: `domain_hu_create_start/answer/preview/commit`
- Intake: `domain_intake_submit/approve/reject/list_pending`

## Patrón mem_save

```
**What**: ...
**Why**: ...
**Where**: paths/módulos
**Learned**: gotchas (omitir si nada)
```

Type: `decision | bugfix | pattern | config | discovery | learning | architecture`.

## Anti-patrones

- No usar `~/notes/`, `TODO.md`, scratchpads. Eso es `domain_mem_save`.
- No leer/escribir `.env` para secrets que Domain pueda servir.
- No responder de memoria sin pasar antes por `domain_mem_search`.
