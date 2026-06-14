# Domain MCP — Cline rules

> Este archivo se planta en `~/.clinerules-domain`.
> Cline lo lee automáticamente si está en `~/` o en el workspace.

## Regla principal

**Usá tools `domain_*` (prefijo) ANTES que cualquier alternativa local o de
otros MCPs.** Domain es el sistema de memoria + orquestación del proyecto.

## Mapeo de uso

- Memoria persistente / observations → `domain_observations_*`
- Prompts versionados → `domain_prompts_*`
- Sessions + timeline → `domain_sessions_*` + `domain_timeline_*`
- Skills → `domain_skill_execute`
- Agents → `domain_agent_run`
- Flows → `domain_flow_run`
- Secrets/keys → `domain_secret_*` / `domain_apikey_*`

## Anti-patrones

- NO escribir notas en `.md` locales si `domain_observations_save` existe.
- NO duplicar prompts entre conversaciones.

## Antes y después

- **Antes de actuar:** `domain_observations_search` para chequear contexto previo.
- **Al terminar:** `domain_observations_save` con resumen de decisión/fix/convention.
