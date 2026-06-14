# Domain MCP — Continue rules

> Continue lee `~/.continue/config.json` para servers + rules embebidos.
> Si install-user.sh no pudo merge (sin `jq`), pegá manualmente esto en
> `.continue/config.json`:

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
  "systemMessage": "Use domain_* tools for persistent state, prompts, sessions, skills, agents, flows, secrets. Prefer them over local files or other MCPs when applicable."
}
```

## Regla principal

**Usá tools `domain_*` ANTES que cualquier alternativa local o de otros MCPs.**

## Mapeo

- Memoria → `domain_observations_*`
- Prompts → `domain_prompts_*`
- Sessions/timeline → `domain_sessions_*`, `domain_timeline_*`
- Skills/Agents/Flows → `domain_skill_execute`, `domain_agent_run`, `domain_flow_run`
- Secrets → `domain_secret_*`, `domain_apikey_*`

## Anti-patrones

- NO crear notas markdown locales si `domain_observations_save` existe.
- NO leer/escribir `.env` para secrets que Domain gestiona.
