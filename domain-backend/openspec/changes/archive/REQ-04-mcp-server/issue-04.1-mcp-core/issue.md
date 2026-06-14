# issue-04.1-mcp-core

**Origen:** `REQ-04-mcp-server`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** desarrollador integrando memoria en agentes de IA
**Quiero** un servidor MCP stdio funcional con transporte JSON-RPC, registro de tools, manejo de errores y shutdown graceful
**Para** que cualquier cliente MCP (Claude Desktop, Cline, etc.) pueda conectarse y usar las tools de memoria

## Criterios de aceptación

```gherkin
Feature: MCP Core Server
  As an MCP client
  I want to connect to the memoria MCP server via stdio transport
  So that I can list and invoke memory tools

  Background:
    Given the memoria MCP server is started with `mem mcp`

  Scenario: Server announces capabilities via initialize
    When the client sends an "initialize" request
    Then the server responds with:
      | protocolVersion | "2024-11-05" |
      | serverInfo.name | "Domain" |
      | serverInfo.version | "<semver>" |
    And the response includes "tools" in capabilities

  Scenario: Client lists available tools
    When the client sends a "tools/list" request
    Then the response is an array of tools
    And each tool has "name", "description", and "inputSchema"
    And the list includes "domain_mem_save", "domain_mem_search", "domain_mem_context"
    And the list includes "domain_mem_timeline", "domain_mem_get_observation"
    And the list includes "domain_mem_stats", "domain_mem_suggest_topic_key"
    And the list includes "mem_update", "domain_mem_delete"
    And the list includes "domain_mem_session_start", "domain_mem_session_end"
    And the list includes "domain_mem_session_summary", "domain_mem_save_prompt"
    And the list includes "domain_mem_capture_passive", "mem_current_project"
    And the list includes "mem_doctor", "mem_judge", "mem_compare"
    And the list includes "mem_merge_projects"

  Scenario: Client calls a valid tool
    When the client sends a "tools/call" request with:
      | name | "mem_current_project" |
      | arguments | {} |
    Then the response has "content" with an array
    And the response has "isError" set to false

  Scenario: Client calls a non-existent tool
    When the client sends a "tools/call" request with:
      | name | "mem_unknown_tool" |
      | arguments | {} |
    Then the response has "isError" set to true
    And the content includes an error message

  Scenario: Transport error on malformed JSON
    When the client sends invalid JSON on stdin
    Then the server writes a JSON-RPC error response to stdout
    And the error code is -32700 (Parse Error)

  Scenario: Transport error on invalid JSON-RPC structure
    When the client sends valid JSON without a "method" field
    Then the server responds with error code -32600 (Invalid Request)

  Scenario: Graceful shutdown on SIGTERM
    When the server receives SIGTERM
    Then it completes the current request
    And it flushes pending writes
    And it exits with code 0

  Scenario: Graceful shutdown on SIGINT
    When the server receives SIGINT (Ctrl+C)
    Then it terminates within 5 seconds
    And it exits with code 0
```

## Análisis breve

- **Qué pide realmente:** Infraestructura base MCP stdio usando `mark3labs/mcp-go`. Transporte stdin/stdout, ciclo JSON-RPC request/response, registro de tools vía `server.SetRequestHandler`, manejo de errores estandarizado, señal de shutdown.
- **Módulos sospechados:** `internal/mcp/` (nuevo paquete), `cmd/mcp/` (nuevo entrypoint o flag `mcp` en CLI existente).
- **Riesgos / dependencias:** `mark3labs/mcp-go` v0.8+ como dependencia externa. El server debe ser un binary standalone. La versión del protocolo MCP debe ser `2024-11-05` (estable).
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
