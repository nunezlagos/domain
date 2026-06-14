# Proposal: issue-12.3-mcp-agent-tools

## Intención

Implementar 9 tools MCP que exponen las capacidades operativas de la plataforma Domain: ejecución de skills, gestión de agentes, orquestación de flows, consulta de cron jobs y búsqueda en knowledge base. Estas tools permiten que agentes y LLM controlen la plataforma programáticamente vía MCP.

## Scope

**Incluye:**

| Tool | Descripción | Args principales |
|---|---|---|
| `domain_skill_execute` | Ejecutar un skill | skill_id, params, context |
| `domain_skill_search` | Buscar skills | query, limit, project |
| `domain_agent_run` | Ejecutar agente | agent_id, input, session_id |
| `domain_agent_create` | Crear agente | name, description, model, system_prompt, skills |
| `domain_flow_run` | Ejecutar flow | flow_id, params, webhook_url |
| `domain_flow_create` | Crear flow | name, steps, triggers |
| `domain_flow_status` | Consultar estado | run_id |
| `domain_cron_list` | Listar cron jobs | project, active |
| `domain_knowledge_search` | Buscar en KB | query, project, limit |

**No incluye:**
- CRUD completo de skills (solo execute + search)
- CRUD completo de agents (solo run + create)
- CRUD completo de flows (solo run + create + status)
- CRUD completo de cron jobs (solo list)
- Dashboard UI (REQ-16-web-ui)

## Enfoque técnico

1. Cada tool es un handler en su propio subdirectorio: `internal/mcp/tools/{domain}/`
2. Los handlers son thin: validan args → llaman a service → formatean respuesta
3. `domain_skill_execute` usa `SkillService.Execute()` que internamente encola en runner
4. `domain_skill_search` usa `SkillService.Search()` con búsqueda full-text
5. `domain_agent_run` usa `AgentService.Run()` que crea domain_agent_run y encola execution
6. `domain_agent_create` usa `AgentService.Create()` con validación de modelo y skills
7. `domain_flow_run` usa `FlowService.Run()` que inicia ejecución asincrónica
8. `domain_flow_create` usa `FlowService.Create()` con validación de steps DAG
9. `domain_flow_status` usa `FlowService.GetRunStatus()` consulta estado en DB
10. `domain_cron_list` usa `CronService.List()` filtrado por proyecto
11. `domain_knowledge_search` usa `KnowledgeService.Search()` con embeddings
12. Tools asincrónicas (domain_skill_execute, domain_agent_run, domain_flow_run) devuelven run_id inmediatamente
13. El cliente puede luego consultar domain_flow_status para ver el resultado

## Riesgos

- **Operaciones asincrónicas:** El cliente MCP espera respuesta síncrona pero la ejecución puede ser larga. Mitigación: devolver run_id inmediatamente, el cliente consulta status después.
- **Permisos:** Un agente externo podría ejecutar actions no autorizadas. Mitigación: las tools verifican project scope y permisos del contexto MCP.
- **Dependencias circulares:** Las tools pueden llamar a servicios que a su vez usan MCP. Mitigación: identificar y documentar dependencias, evitar loops.
- **Rate limiting:** Clientes MCP podrían saturar la plataforma. Mitigación: rate limiting por tool, cola de ejecución.

## Testing

- Unit: cada tool handler con service mock
- Unit: validación de argumentos (requeridos, tipos, formatos)
- Integration: tools reales contra servicios reales
- Integration: domain_skill_execute con skill real y runner mock
- Integration: domain_flow_run + domain_flow_status ciclo completo
- Integration: domain_knowledge_search con documentos indexados
