# HU-04.2-mcp-save-tools

**Origen:** `REQ-04-mcp-server`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** agente de IA usando MCP
**Quiero** herramientas para guardar, actualizar, eliminar observaciones, sugerir topic_keys y guardar prompts
**Para** persistir mi memoria contextual y mantener el grafo de conocimiento actualizado

## Criterios de aceptación

```gherkin
Feature: MCP Save Tools

  Background:
    Given the MCP server is running
    And the "domain_mem_save" tool is registered
    And the "mem_update" tool is registered
    And the "domain_mem_delete" tool is registered
    And the "domain_mem_suggest_topic_key" tool is registered
    And the "domain_mem_save_prompt" tool is registered

  Scenario: Save a new observation with all fields
    When the client calls "domain_mem_save" with arguments:
      | title       | "feat: implement dark mode" |
      | type        | "decision" |
      | content     | "We chose Tailwind dark mode over CSS vars for DX" |
      | topic_key   | "frontend-dark-mode" |
      | scope       | "project" |
      | project     | "my-app" |
    Then the response has "isError" set to false
    And the content includes an "id" field
    And the id is a positive integer

  Scenario: Save an observation with minimal fields
    When the client calls "domain_mem_save" with arguments:
      | title   | "quick note" |
      | content | "remember to review PR #42" |
    Then the response has "isError" set to false
    And the observation is saved with:
      | type      | "context" (default) |
      | scope     | "personal" (default) |
      | topic_key | "quick-note" (generated) |

  Scenario: Save with conflict detection
    When the client calls "domain_mem_save" with arguments:
      | title   | "feat: implement dark mode" |
      | type    | "decision" |
      | content | "Different content but similar title" |
    Then the response includes a "candidates" array
    And candidates contains existing observations with similar titles
    And the response has "isError" set to false

  Scenario: Save with capture_prompt and session_id
    When the client calls "domain_mem_save" with arguments:
      | title          | "refactor: extract auth middleware" |
      | content        | "Moved JWT validation to its own middleware" |
      | capture_prompt | "What architectural decisions were made?" |
      | session_id     | "sess_abc123" |
    Then the observation is saved
    And the observation has the capture_prompt field set
    And the observation has the session_id field set

  Scenario: Update an existing observation
    Given an observation with id=42 exists
    When the client calls "mem_update" with arguments:
      | id      | 42 |
      | content | "Updated content with newer reasoning" |
    Then the response has "isError" set to false
    And the observation with id=42 now has the updated content
    And other fields of id=42 remain unchanged

  Scenario: Update non-existent observation
    When the client calls "mem_update" with arguments:
      | id      | 99999 |
      | content | "anything" |
    Then the response has "isError" set to true
    And the error message indicates "observation not found"

  Scenario: Soft delete an observation
    Given an observation with id=7 exists
    When the client calls "domain_mem_delete" with arguments:
      | id         | 7 |
      | hard_delete | false |
    Then the response has "isError" set to false
    And the observation with id=7 is marked as deleted
    And a normal "domain_mem_search" does not return it

  Scenario: Hard delete an observation
    Given an observation with id=8 exists
    When the client calls "domain_mem_delete" with arguments:
      | id         | 8 |
      | hard_delete | true |
    Then the response has "isError" set to false
    And the observation with id=8 is permanently removed

  Scenario: Suggest topic key from type and title
    When the client calls "domain_mem_suggest_topic_key" with arguments:
      | type  | "decision" |
      | title | "We chose PostgreSQL over MongoDB for consistency" |
    Then the response contains a "topic_key" field
    And the topic_key is "postgresql-over-mongodb-for-consistency"

  Scenario: Suggest topic key with multi-language title
    When the client calls "domain_mem_suggest_topic_key" with arguments:
      | type  | "fix" |
      | title | "Corregido el bug de autenticación OAuth" |
    Then the topic_key is slugified to "corregido-el-bug-de-autenticacion-oauth"

  Scenario: Save user prompt
    When the client calls "domain_mem_save_prompt" with arguments:
      | content   | "How do I implement WebSocket reconnection?" |
      | session_id | "sess_abc123" |
    Then the response has "isError" set to false
    And the prompt is saved with a timestamp

  Scenario: Save prompt feeds process-local context
    When the client calls "domain_mem_save_prompt" with any content
    Then the prompt is also appended to the process-local prompt ring buffer
    And the buffer retains the last N prompts

  Scenario: Save with invalid type
    When the client calls "domain_mem_save" with arguments:
      | title | "test" |
      | type  | "invalid_type_here" |
      | content | "test content" |
    Then the response has "isError" set to true
    And the error message indicates valid type values
```

## Análisis breve

- **Qué pide realmente:** 5 tools MCP que exponen las operaciones CRUD del motor Engram: save, update, delete, suggest_topic_key (helpers) y save_prompt (logging de prompts con contexto local).
- **Módulos sospechados:** `internal/engram/` (si existe) o `internal/mcp/tools/save.go` con llamadas al core engine. El proceso-local ring buffer vive en `internal/mcp/context.go`.
- **Riesgos / dependencias:** Conflict detection requiere integración con REQ-10 (lexical scan). save_prompt ring buffer tiene tamaño fijo (N=100). update necesita validar que el ID existe antes.
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
