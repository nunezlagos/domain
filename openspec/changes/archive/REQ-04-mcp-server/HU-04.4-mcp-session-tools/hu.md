# HU-04.4-mcp-session-tools

**Origen:** `REQ-04-mcp-server`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** agente de IA operando en una sesión de trabajo
**Quiero** herramientas para iniciar, cerrar, resumir sesiones y capturar aprendizaje pasivo
**Para** mantener contexto coherente entre interacciones y construir memoria episódica

## Criterios de aceptación

```gherkin
Feature: MCP Session Tools

  Background:
    Given the MCP server is running

  Scenario: Start a new session
    When the client calls "domain_mem_session_start" with arguments:
      | id        | "sess_abc123" |
      | directory | "/home/user/projects/my-app" |
    Then the response has "isError" set to false
    And a new session is registered with:
      | id        | "sess_abc123" |
      | directory | "/home/user/projects/my-app" |
      | status    | "active" |
      | started_at | <current timestamp> |

  Scenario: Start a session with the same id is idempotent
    Given a session with id "sess_abc123" exists
    When the client calls "domain_mem_session_start" with arguments:
      | id | "sess_abc123" |
    Then the response has "isError" set to false
    And the existing session is returned (not duplicated)

  Scenario: End a session
    Given a session with id "sess_abc123" exists and is active
    When the client calls "domain_mem_session_end" with arguments:
      | id | "sess_abc123" |
    Then the response has "isError" set to false
    And the session status is now "ended"
    And "ended_at" is set

  Scenario: End a session with optional summary
    Given a session with id "sess_abc123" exists
    When the client calls "domain_mem_session_end" with arguments:
      | id      | "sess_abc123" |
      | summary | "Reviewed 3 PRs, fixed 2 bugs, refactored auth middleware" |
    Then the session summary is saved
    And the session status is "ended"

  Scenario: End a non-existent session
    When the client calls "domain_mem_session_end" with arguments:
      | id | "sess_nonexistent" |
    Then the response has "isError" set to true
    And the error indicates "session not found"

  Scenario: End an already ended session
    Given a session with id "sess_abc123" has status "ended"
    When the client calls "domain_mem_session_end" with arguments:
      | id | "sess_abc123" |
    Then the response has "isError" set to true
    And the error indicates "session already ended"

  Scenario: Save a comprehensive session summary
    Given a session with id "sess_abc123" exists
    When the client calls "domain_mem_session_summary" with arguments:
      | id      | "sess_abc123" |
      | content | "## Accomplished\n- Fixed auth bug\n- Added rate limiting\n\n## Decisions\n- Use Redis for rate limiting\n\n## Next Steps\n- Add tests for rate limiter" |
    Then the response has "isError" set to false
    And the session summary is stored as a structured observation
    And the session has "has_summary" set to true

  Scenario: Capture passive learnings from tool output
    When the client calls "domain_mem_capture_passive" with arguments:
      | content | "Discovered that the JWT library v3 has a breaking change in VerifyToken signature" |
      | session_id | "sess_abc123" |
    Then the response has "isError" set to false
    And a new observation of type "pattern" is created
    And the observation is linked to the session

  Scenario: Capture passive without session
    When the client calls "domain_mem_capture_passive" with arguments:
      | content | "Important finding without session context" |
    Then the response has "isError" set to false
    And the observation is created without session linkage
    And the observation type defaults to "context"

  Scenario: Session summary generates observations
    When the client calls "domain_mem_session_summary" with structured markdown
    Then each "Accomplished" item becomes a separate observation
    And each "Decision" item becomes a separate observation of type "decision"
    And each "Next Steps" item becomes a separate observation of type "context"
```

## Análisis breve

- **Qué pide realmente:** 4 tools MCP para el ciclo de vida de sesiones: `domain_mem_session_start`, `domain_mem_session_end`, `domain_mem_session_summary`, `domain_mem_capture_passive`. Las sesiones agrupan observaciones y permiten contexto temporal.
- **Módulos sospechados:** `internal/mcp/tools/session.go`, `internal/engram/session.go` (dominio de sesiones).
- **Riesgos / dependencias:** session_start debe ser idempotente. session_end debe validar que la sesión existe y está activa. El parseo de markdown estructurado en session_summary es heurístico.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
