# REQ-12-mcp-server: Servidor MCP bidireccional: tools para memorias, skills, agentes, flujos. Consumo de MCPs externos. Project resolution. Error envelopes.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F2, F3

## Descripción

Servidor MCP bidireccional: tools para memorias, skills, agentes, flujos. Consumo de MCPs externos. Project resolution. Error envelopes.

## Criterios de éxito

- Servidor MCP stdio funcional con initialize, tools/list, tools/call y graceful shutdown
- 12 tools de memoria operativas contra Postgres
- 9 tools de plataforma (skills, agentes, flows, cron, knowledge)
- MCP Hub que conecta, descubre y expone servidores MCP externos como skills nativos
- Setup automático de Domain como MCP server en Claude Code, OpenCode, Codex y Cline
- `.ai/directives.md` generado con instrucciones para preferir tools Domain
- Resilience production-grade: timeout + circuit breaker + cache LRU last-known-good + degraded responses + retry transitorio + config en BD con reload via NOTIFY

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-12.1-mcp-core-stdio | proposed | Core MCP stdio con mark3labs/mcp-go: JSON-RPC, tool registry, errores, shutdown |
| issue-12.2-mcp-memory-tools | proposed | 12 tools MCP de memoria (domain_mem_save, domain_mem_search, etc.) respaldadas por Postgres |
| issue-12.3-mcp-agent-tools | proposed | 9 tools MCP de plataforma (domain_skill_execute, domain_agent_run, domain_flow_create, etc.) |
| issue-12.4-mcp-bidirectional | proposed | MCP Hub bidireccional: consume servidores MCP externos y los expone como skills |
| issue-12.5-agent-setup | proposed | Setup de Domain como MCP server en Claude Code, OpenCode, Codex, Cline. .ai/ folder con directivas. Safe files protection. |
| issue-12.6-mcp-tool-resilience | proposed | Timeout + circuit breaker + cache LRU + degraded responses + retry transitorio + config en BD hot-reload |
