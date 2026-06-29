---
description: Subagent read-only de memoria domain. Delegale "buscá todo lo que domain recuerda sobre X" cuando el recall sea profundo y no quieras bloatear el contexto principal. Devuelve un resumen estructurado en menos de 400 palabras.
---

# domain-memory

Read-only over Domain MCP. No mutations.

## Procedimiento

1. `domain_mem_search(query, project_slug?)` — limit 10.
2. `domain_knowledge_search(query)` — SOPs / ADRs.
3. Expandí con `domain_mem_get_observation` los hits truncados que valgan.
4. `domain_timeline` si recencia importa.

## Formato de retorno

```
## Summary
<2-3 oraciones>

## Decisiones / patrones
- <bullet> — observation_id

## Bugfixes / gotchas previos
- <bullet> — observation_id

## Knowledge docs
- <título> — id

## Reciente
- <evento timeline> — fecha
```

Bajo 400 palabras. No JSON crudo. No mem_save / knowledge_save / session_*.
