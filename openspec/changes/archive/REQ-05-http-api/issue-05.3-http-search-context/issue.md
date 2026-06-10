# issue-05.3-http-search-context

**Origen:** `REQ-05-http-api`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador integrando memoria en un agente de IA
**Quiero** endpoints HTTP para búsqueda full-text, timeline de contexto y retrieval de contexto por proyecto
**Para** que el agente pueda consultar memorias relevantes sin depender de la CLI

## Criterios de aceptación

```gherkin
Feature: HTTP Search, Timeline, and Context API
  As an HTTP client
  I want to search observations, view timeline, and retrieve project context
  So that I can integrate memory retrieval into AI agents and external tools

  Background:
    Given the memoria HTTP server is running on localhost:7437
    And there are observations with various titles and content indexed in FTS5

  Scenario: Search by query string
    When I send a GET request to /search?q=hello
    Then the response status is 200
    And the response body is an array
    And each result contains "id", "title", and "content"
    And results match the FTS5 query "hello"

  Scenario: Search with type filter
    When I send a GET request to /search?q=hello&type=decision
    Then all results have type "decision"

  Scenario: Search with project filter
    When I send a GET request to /search?q=hello&project=myapp
    Then all results have project "myapp"

  Scenario: Search with scope filter
    When I send a GET request to /search?q=hello&scope=project
    Then all results have scope "project"

  Scenario: Search with limit
    When I send a GET request to /search?q=hello&limit=5
    Then the response has at most 5 items

  Scenario: Search without query returns 400
    When I send a GET request to /search
    Then the response status is 400

  Scenario: Search with empty query returns 400
    When I send a GET request to /search?q=
    Then the response status is 400

  Scenario: Get timeline for an observation
    Given an observation with id "1" exists with before/after context
    When I send a GET request to /timeline?observation_id=1&before=3&after=3
    Then the response status is 200
    And the response body contains "center" with the observation
    And the response body contains "before" with at most 3 observations
    And the response body contains "after" with at most 3 observations

  Scenario: Timeline with only before
    When I send a GET request to /timeline?observation_id=1&before=5
    Then the response contains "center"
    And the response contains "before" with at most 5 items
    And the response contains "after" as empty array

  Scenario: Timeline for non-existent observation returns 404
    When I send a GET request to /timeline?observation_id=9999
    Then the response status is 404

  Scenario: Get context for a project
    When I send a GET request to /context?project=myapp&scope=project
    Then the response status is 200
    And the response body contains "project" with value "myapp"
    And the response body contains "observations" as an array
    And each observation has scope "project"

  Scenario: Get context without project returns 400
    When I send a GET request to /context
    Then the response status is 400

  Scenario: Get context returns only non-deleted observations
    When there are soft-deleted observations for project "myapp"
    And I send a GET request to /context?project=myapp
    Then deleted observations are not included
```

## Análisis breve

- **Qué pide realmente:** 3 endpoints: (1) GET /search con FTS5 full-text search y filtros opcionales type/project/scope/limit, (2) GET /timeline con observaciones alrededor de una central (before/after), (3) GET /context con observaciones de un projecto filtradas por scope. Todos deben excluir soft-deleted.
- **Módulos sospechados:** `internal/api/search.go` (handlers), `internal/store/search.go` (FTS5 queries, issue-01.3), `internal/store/observation.go` (context/timeline queries)
- **Riesgos / dependencias:** Depende de issue-01.3 (FTS5 search) y issue-01.2 (observation queries). FTS5 debe estar configurado con triggers.
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
