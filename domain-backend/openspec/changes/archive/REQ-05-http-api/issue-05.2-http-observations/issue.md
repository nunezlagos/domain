# issue-05.2-http-observations

**Origen:** `REQ-05-http-api`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador que consume la API de memoria
**Quiero** endpoints REST para crear, leer, actualizar y eliminar observaciones, con detección de conflict candidates y captura pasiva
**Para** gestionar el contenido de memoria desde herramientas externas sin usar la CLI

## Criterios de aceptación

```gherkin
Feature: HTTP Observations API
  As an HTTP client
  I want full CRUD for observations with smart conflict detection
  So that I can manage memory content programmatically

  Background:
    Given the memoria HTTP server is running on localhost:7437
    And a session "s1" exists

  Scenario: Create an observation
    When I send a POST request to /observations with:
      | session_id | "s1"                       |
      | title      | "API Design Decision"      |
      | content    | "Use REST over GraphQL"    |
      | type       | "decision"                 |
      | project    | "myapp"                    |
      | scope      | "project"                  |
    Then the response status is 201
    And the response body contains "id"
    And the response body contains "title" with value "API Design Decision"

  Scenario: Create observation with conflict candidate detection
    Given an existing observation with normalized_hash "abc123"
    When I send a POST request to /observations with content similar to the existing one
    Then the response status is 201
    And the response body contains "conflict_candidate"
    And "conflict_candidate" contains "existing_id"
    And "conflict_candidate" contains "similarity_score"

  Scenario: Create observation with minimal fields
    When I send a POST request to /observations with:
      | session_id | "s1"                    |
      | content    | "Quick note"            |
    Then the response status is 201

  Scenario: Create observation without session_id returns 400
    When I send a POST request to /observations with:
      | content | "orphan" |
    Then the response status is 400

  Scenario: Get recent observations
    When I send a GET request to /observations/recent
    Then the response status is 200
    And the response body is an array
    And items are ordered by created_at descending

  Scenario: Get recent observations with filters
    When I send a GET request to /observations/recent?limit=5&project=myapp&type=decision
    Then the response array has at most 5 items
    And each item has project "myapp"
    And each item has type "decision"

  Scenario: Get observation by ID
    Given an observation with id "1" exists
    When I send a GET request to /observations/1
    Then the response status is 200
    And the response body contains "id" with value 1

  Scenario: Get non-existent observation returns 404
    When I send a GET request to /observations/9999
    Then the response status is 404

  Scenario: Update an observation
    Given an observation with id "1" exists
    When I send a PATCH request to /observations/1 with:
      | title       | "Updated Title"         |
      | content     | "Updated content"       |
      | revision_count | 2                    |
    Then the response status is 200
    And the response body contains "title" with value "Updated Title"
    And the response body contains "revision_count" with value 2

  Scenario: Update non-existent observation returns 404
    When I send a PATCH request to /observations/9999 with:
      | title | "Nope" |
    Then the response status is 404

  Scenario: Soft delete an observation
    Given an observation with id "1" exists
    When I send a DELETE request to /observations/1
    Then the response status is 204
    And a subsequent GET /observations/1 returns 404

  Scenario: Hard delete an observation
    Given an observation with id "2" exists
    When I send a DELETE request to /observations/2?hard=true
    Then the response status is 204
    And a subsequent GET /observations/2 returns 404

  Scenario: Passive capture creates observation silently
    When I send a POST request to /observations/passive with:
      | session_id | "s1"                                |
      | content    | "background log captured passively" |
      | source     | "tool_output"                       |
    Then the response status is 201
    And the response body contains "id"
```

## Análisis breve

- **Qué pide realmente:** 6 endpoints HTTP para CRUD de observaciones + captura pasiva + conflict candidate detection. POST /observations detecta duplicados vía normalized_hash y retorna sugerencia. DELETE tiene soft (default) y hard (?hard=true). GET /observations/recent acepta filtros.
- **Módulos sospechados:** `internal/api/observations.go` (handlers), `internal/store/observation.go` (repo, puede existir de issue-01.2), `internal/dedup/` (conflict detection de issue-01.4)
- **Riesgos / dependencias:** Depende de issue-01.2 (observation CRUD), issue-01.4 (deduplicación/conflict detection), issue-02.3 (passive capture)
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
