# issue-01.3-fts5-search

**Origen:** `REQ-01-core-memory-store`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** buscar observaciones mediante búsqueda de texto completo
**Para** encontrar rápidamente decisiones, bugs y patrones previos sin escanear todo el almacén manualmente

## Criterios de aceptación (Gherkin)

```gherkin
Feature: FTS5 full-text search

  Background:
    Given the store has observations with titles and contents
    And an FTS5 virtual table is configured on the observations table
    And soft-deleted observations exist in the database

  Scenario: Basic keyword search matches title and content
    When I search for "handler"
    Then results include observations where title contains "handler"
    And results include observations where content contains "handler"
    And results are ordered by relevance descending

  Scenario: Query sanitization prevents FTS5 syntax errors
    When I search for "decision - bugfix"
    Then the query is sanitized to '"decision" "bugfix"'
    And no FTS5 syntax error is raised

  Scenario: Search with special FTS5 characters is escaped
    When I search for "don't stop ^NEAR"
    Then the query is sanitized to escape special chars: ^ " * : ~ ( ) + -
    And each token is wrapped in double quotes

  Scenario: Filter results by type
    When I search for "handler" with type "decision"
    Then only observations of type "decision" are returned
    And results match the search keyword

  Scenario: Filter results by project
    When I search for "handler" with project "Domain"
    Then only observations belonging to project "Domain" are returned

  Scenario: Filter results by scope
    When I search for "handler" with scope "project"
    Then only observations with scope "project" are returned

  Scenario: Combined filters with type, project and scope
    When I search for "handler" with type "fix", project "Domain" and scope "project"
    Then results satisfy all three filters simultaneously

  Scenario: Empty query returns error
    When I search with an empty query ""
    Then the system returns an error: "query cannot be empty"
    And no database query is executed

  Scenario: Soft-deleted observations are excluded from results
    When I search for "handler"
    Then no soft-deleted observations appear in results

  Scenario: Results include full observation metadata
    When I search for "handler"
    Then each result includes: id, title, content, type, project, scope, created_at
    And each result includes a relevance score

  Scenario: Pagination with limit and offset
    When I search for "handler" with limit 10 and offset 0
    Then at most 10 results are returned
    When I search with limit 10 and offset 10
    Then the next page of results is returned

  Scenario: Search prompts table separately
    When I search for "refactor" in prompts
    Then results come from the prompts_fts index
    And results include prompt metadata: id, content, created_at

  Scenario: Snippet/highlight in results
    When I search for "handler" with snippets enabled
    Then each result includes a highlighted snippet of matching content
```

## Análisis breve

- **Qué pide realmente:** Implementar FTS5 full-text search sobre las tablas `observations` y `user_prompts` con sanitización de queries, filtros combinados (type, project, scope), snippets/paginación, y exclusión de soft-deletes.
- **Módulos sospechados:** `store/sqlite/` (queries), `store/` (interface), `models/` (result types)
- **Riesgos / dependencias:** Depende del schema de issue-01.1 (tablas `observations_fts` y `prompts_fts`). La sanitización es crítica para evitar errores FTS5 con caracteres especiales.
- **Esfuerzo tentativo:** L

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
