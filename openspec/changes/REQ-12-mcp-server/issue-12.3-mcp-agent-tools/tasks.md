# Tasks: issue-12.3-mcp-agent-tools

## Backend

- [ ] Crear `internal/mcp/tools/skill/tool_skill_execute.go`: handler MCP para domain_skill_execute
- [ ] Crear `internal/mcp/tools/skill/tool_skill_search.go`: handler MCP para domain_skill_search
- [ ] Crear `internal/mcp/tools/agent/tool_agent_run.go`: handler MCP para domain_agent_run
- [ ] Crear `internal/mcp/tools/agent/tool_agent_create.go`: handler MCP para domain_agent_create
- [ ] Crear `internal/mcp/tools/flow/tool_flow_run.go`: handler MCP para domain_flow_run
- [ ] Crear `internal/mcp/tools/flow/tool_flow_create.go`: handler MCP para domain_flow_create
- [ ] Crear `internal/mcp/tools/flow/tool_flow_status.go`: handler MCP para domain_flow_status
- [ ] Crear `internal/mcp/tools/cron/tool_cron_list.go`: handler MCP para domain_cron_list
- [ ] Crear `internal/mcp/tools/knowledge/tool_knowledge_search.go`: handler MCP para domain_knowledge_search
- [ ] Implementar SkillService.Execute() con encolado async en runner (o mock temporal)
- [ ] Implementar SkillService.Search() con búsqueda full-text en skills
- [ ] Implementar AgentService.Run() con creación de domain_agent_run + encolado
- [ ] Implementar AgentService.Create() con validación de modelo LLM y skills
- [ ] Implementar FlowService.Run() con inicio de ejecución asincrónica
- [ ] Implementar FlowService.Create() con validación de DAG de steps
- [ ] Implementar FlowService.GetRunStatus() con consulta a DB
- [ ] Implementar CronService.List() con filtros project + active
- [ ] Implementar KnowledgeService.Search() con embeddings + full-text
- [ ] Registrar las 9 tools en `cmd/domain-mcp/main.go`
- [ ] Definir inputSchema para cada tool con JSON Schema
- [ ] Implementar validación de argumentos (requeridos, tipos, formatos)
- [ ] Implementar rate limiting por tool en MCPServer

## Frontend

- [ ] (No aplica)

## Tests

- [ ] Test unitario: domain_skill_execute handler valida skill_id
- [ ] Test unitario: domain_skill_execute devuelve run_id con Service mock
- [ ] Test unitario: domain_skill_search handler con resultados mock
- [ ] Test unitario: domain_agent_run handler con validación de agent_id
- [ ] Test unitario: domain_agent_create handler con validación de model
- [ ] Test unitario: domain_agent_create con skills inválidos devuelve error
- [ ] Test unitario: domain_flow_run handler con validación de flow_id
- [ ] Test unitario: domain_flow_create handler con validación de steps
- [ ] Test unitario: domain_flow_status handler con estados running/success/failed
- [ ] Test unitario: domain_cron_list handler con filtros
- [ ] Test unitario: domain_knowledge_search handler con resultados mock
- [ ] Test unitario: validación de argumentos requeridos por tool
- [ ] Test integración: domain_skill_execute + domain_flow_status ciclo async
- [ ] Test integración: domain_agent_create + domain_agent_run ciclo completo
- [ ] Test integración: domain_flow_create + domain_flow_run + domain_flow_status
- [ ] Sabotaje: domain_flow_status con run_id inexistente → error
- [ ] Sabotaje: domain_agent_create sin name → error validación

## Cierre

- [ ] Verificación manual: invocar cada tool desde cliente MCP
- [ ] Suite verde: `go test ./internal/mcp/tools/...`
- [ ] Documentar cada tool MCP con ejemplos de uso en docs/mcp-tools.md
