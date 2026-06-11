# Tasks: issue-12.3-mcp-agent-tools

> Nota de estructura: tools en `internal/mcp/server/` (server.go +
> catalog_tools.go) en lugar de un paquete por tool; services por feature
> (clean-architecture.md).

## Backend — tools MCP

- [x] domain_skill_execute → catalog_tools.go (sync/async vía ExecutionService 05.5, validación input_schema, log persistente) — 2026-06-10
- [x] domain_skill_search → server.go (+ skill_list, skill_get)
- [x] domain_agent_run → server.go
- [x] domain_agent_create → catalog_tools.go (provider/model/system_prompt + skills validados) — 2026-06-10
- [x] domain_flow_run → server.go
- [x] domain_flow_create → catalog_tools.go (Spec.Validate: tipos, DAG, error policies) — 2026-06-10
- [x] domain_flow_status → orchestrate_tools.go
- [x] domain_cron_list → catalog_tools.go (next_run/last_run/enabled) — 2026-06-10
- [x] domain_knowledge_search → server.go (+ knowledge_save, knowledge_get)

## Backend — services

- [x] Skill execute → skillsvc.ExecutionService (issue-05.5: sync/async + polling)
- [x] Skill search → skill.Service.SearchHybrid
- [x] Agent run → agentrunner.Runner + agent_runs
- [x] Agent create → agent.Service.Create (valida provider, skills existentes)
- [x] Flow run → flowrunner.Runner
- [x] Flow create → flow.Service.Create con Spec.Validate (DAG Kahn)
- [x] Flow run status → flow.Service.GetRun + GetRunSteps
- [x] Cron list → cron.Service.List (filtro org; project N/A: crons son org-scoped)
- [x] Knowledge search → knowledge.Service (híbrida)
- [x] Registrar tools → Tools() + registerCatalogTools; wiring en cmd/domain-mcp (SkillExecution + Crons en Deps) — 2026-06-10
- [x] inputSchema por tool → mcp.With* + Required
- [x] Validación de argumentos → handlers con type assertions + ToolResultError
- [x] Rate limiting por tool → ResilientWrapper (issue-12.6 budgets)

## Tests

- [x] skill_execute valida skill y params → TestMCP_SkillExecute (válido + required faltante + slug inexistente) — 2026-06-10
- [x] skill_execute devuelve execution con status → mismo test (integración real, no mock — política del repo)
- [x] skill_search → suite MCP existente
- [x] agent_run validación → suite MCP existente
- [x] agent_create validación de provider/model → TestMCP_CatalogTools_EndToEnd — 2026-06-10
- [x] agent_create duplicado/inválido → mismo test (slug dup → error) — 2026-06-10
- [x] flow_run validación → suite MCP existente
- [x] flow_create validación de steps → TestMCP_CatalogTools_EndToEnd — 2026-06-10
- [x] flow_status estados → tests de orchestrate_tools existentes
- [x] cron_list → TestMCP_CatalogTools_EndToEnd (lista vacía con total) — 2026-06-10
- [x] knowledge_search → suite MCP existente
- [x] validación de args requeridos → cubierta por cada handler test
- [x] Integración execute + status async → TestExecute_Async_PollUntilCompleted (service layer, issue-05.5)
- [x] Integración agent create + run → create cubierto; run con LLM real requiere provider (cubierto por agentrunner integration)
- [x] Integración flow create + run + status → flow tools + TestFlowRunAPI_Lifecycle (API)
- [x] Sabotaje: flow con ciclo → rechazado → TestMCP_CatalogTools_EndToEnd — 2026-06-10
- [x] Sabotaje: agent_create sin campos → handler error (slug/name/provider/model requeridos)

## Cierre

- [x] Verificación manual → mcptest in-process (mismo protocolo que cliente real)
- [x] Suite verde → 2026-06-10
- [x] Documentación → descriptions por tool (consumidas por clients MCP)
