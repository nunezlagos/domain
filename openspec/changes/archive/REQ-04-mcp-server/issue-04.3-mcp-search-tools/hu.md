# issue-04.3-mcp-search-tools

**Origen:** `REQ-04-mcp-server`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** agente de IA usando MCP
**Quiero** buscar, navegar y examinar mi memoria existente con FTS5, contexto cronológico, timeline y estadísticas
**Para** recuperar decisiones pasadas y entender el estado actual del proyecto

## Criterios de aceptación

```gherkin
Feature: MCP Search Tools

  Background:
    Given the MCP server is running
    And the database contains observations with various types, projects, and scopes

  Scenario: Search observations by text query
    When the client calls "domain_mem_search" with arguments:
      | query | "dark mode implementation" |
    Then the response has "isError" set to false
    And the content includes matching observations
    And each result contains "id", "title", "type", "content", "created_at"

  Scenario: Search filters by type
    When the client calls "domain_mem_search" with arguments:
      | query | "authentication" |
      | type  | "decision" |
    Then all returned observations have type "decision"

  Scenario: Search filters by project
    When the client calls "domain_mem_search" with arguments:
      | query   | "refactor" |
      | project | "my-app" |
    Then all returned observations have project "my-app"

  Scenario: Search filters by scope
    When the client calls "domain_mem_search" with arguments:
      | query | "personal note" |
      | scope | "personal" |
    Then all returned observations have scope "personal"

  Scenario: Search with limit
    When the client calls "domain_mem_search" with arguments:
      | query | "the" |
      | limit | 3 |
    Then at most 3 observations are returned

  Scenario: Search in all projects mode
    When the client calls "domain_mem_search" with arguments:
      | query        | "cross-project" |
      | all_projects | true |
    Then observations from multiple projects are returned

  Scenario: Search results include conflict annotations
    Given some observations have pending conflicts
    When the client calls "domain_mem_search" with any query
    Then results may include a "conflicts" field on conflicted observations
    And the conflict annotation includes "conflict_id" and "status"

  Scenario: Get recent context
    When the client calls "domain_mem_context" with arguments:
      | project | "my-app" |
      | scope   | "project" |
    Then the response includes recent observations for the project
    And the response includes session information
    And results are ordered by recency (newest first)

  Scenario: Context defaults to current project
    When the client calls "domain_mem_context" with no arguments
    Then the response uses the current project from working directory
    And the response includes a "project" field

  Scenario: Timeline around an observation
    Given an observation with id=15 exists
    When the client calls "domain_mem_timeline" with arguments:
      | observation_id | 15 |
      | before         | 3 |
      | after          | 3 |
    Then the response includes 3 observations before id=15
    And the response includes 3 observations after id=15
    And the observations are ordered chronologically

  Scenario: Timeline at the edge of history
    Given the first observation has id=1
    When the client calls "domain_mem_timeline" with arguments:
      | observation_id | 1 |
      | before         | 5 |
      | after          | 5 |
    Then before has fewer than 5 entries (edge of history)
    And after has up to 5 entries

  Scenario: Get full observation by ID
    Given an observation with id=42 exists with very long content
    When the client calls "domain_mem_get_observation" with arguments:
      | id | 42 |
    Then the response includes the full untruncated content
    And all fields are returned (title, type, content, topic_key, scope, project, created_at, updated_at)

  Scenario: Get non-existent observation
    When the client calls "domain_mem_get_observation" with arguments:
      | id | 99999 |
    Then the response has "isError" set to true
    And the error indicates "observation not found"

  Scenario: Get system statistics
    When the client calls "domain_mem_stats" with no arguments
    Then the response includes:
      | total_observations    | <integer> |
      | total_sessions        | <integer> |
      | total_prompts         | <integer> |
      | unique_projects       | <integer> |
      | oldest_observation    | <timestamp> |
      | newest_observation    | <timestamp> |
      | storage_size_bytes    | <integer> |
```

## Análisis breve

- **Qué pide realmente:** 5 tools MCP de lectura: `domain_mem_search` (FTS5), `domain_mem_context` (resumen reciente), `domain_mem_timeline` (entorno cronológico), `domain_mem_get_observation` (por ID), `domain_mem_stats` (estadísticas). Anotaciones de conflicto en resultados.
- **Módulos sospechados:** `internal/mcp/tools/search.go`, `internal/engram/` (store queries), posible integración con FTS5 de SQLite.
- **Riesgos / dependencias:** FTS5 requiere SQLite compilado con FTS5. Las anotaciones de conflicto dependen de REQ-10. Timeline necesita índices por created_at.
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
