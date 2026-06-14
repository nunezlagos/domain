# Domain MCP — Continue rules

> Continue lee `~/.continue/config.json` para servers + systemMessage.
> Si install-user.sh no pudo merge (sin `jq`), pegá manualmente:

```json
{
  "experimental": {
    "modelContextProtocolServers": [
      {
        "transport": {
          "type": "http",
          "url": "https://YOUR-VPS-URL/mcp",
          "headers": {
            "Authorization": "Bearer YOUR-API-KEY"
          }
        }
      }
    ]
  },
  "systemMessage": "Domain MCP server registered (prefix domain_*). Each turn: 1) classify message with domain_orchestrate (intent=chat|idea|feature|fix|refactor|hotfix|doc|rfc|analysis); 2) before responding, call domain_mem_search; 3) work; 4) close turn with domain_mem_save for non-obvious decisions. Use domain_* tools instead of local notes, scratchpads, or .env reads for secrets."
}
```

## Protocolo (cada turno)

1. `domain_orchestrate raw_text=<mensaje>` + `project_slug` → intent + plan.
2. `domain_mem_search` antes de responder.
3. Trabajar (Edit/Read son nativos de Continue).
4. `domain_mem_save` antes de cerrar.

## Tools clave

- Memoria: `domain_mem_save`, `domain_mem_search`, `domain_mem_context`
- Orquestación: `domain_orchestrate`, `domain_orchestrate_phase_result`
- Sesiones: `domain_session_start/end/active`, `domain_timeline`
- Catálogo: `domain_project_list/create`, `domain_client_list/get/create/update`
- Knowledge: `domain_knowledge_save/search/get`
- Skills/agents/flows: `domain_skill_execute`, `domain_agent_run`, `domain_flow_run`

## Anti-patrones

- No usar `~/notes/`, `TODO.md` o scratchpads para estado persistente.
- No leer `.env` para secrets que Domain pueda servir.
- No responder sin antes hacer `domain_mem_search`.
