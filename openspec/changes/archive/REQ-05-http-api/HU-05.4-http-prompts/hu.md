# HU-05.4-http-prompts

**Origen:** `REQ-05-http-api`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** desarrollador que registra prompts de usuario en memoria
**Quiero** endpoints HTTP para guardar, listar, buscar y eliminar prompts
**Para** que los agentes puedan persistir sus consultas y recuperar historial desde la API REST

## Criterios de aceptación

```gherkin
Feature: HTTP Prompts API
  As an HTTP client
  I want to manage user prompts via REST endpoints
  So that I can persist and retrieve AI agent queries programmatically

  Background:
    Given the memoria HTTP server is running on localhost:7437
    And a session "s1" exists

  Scenario: Save a prompt
    When I send a POST request to /prompts with:
      | session_id | "s1"                              |
      | content    | "What is the architecture?"       |
      | project    | "myapp"                           |
    Then the response status is 201
    And the response body contains "id"
    And the response body contains "content" with value "What is the architecture?"
    And the response body contains "created_at"

  Scenario: Save prompt with minimal fields
    When I send a POST request to /prompts with:
      | session_id | "s1"                |
      | content    | "minimal prompt"    |
    Then the response status is 201

  Scenario: Save prompt without content returns 400
    When I send a POST request to /prompts with:
      | session_id | "s1" |
    Then the response status is 400

  Scenario: Save prompt without session_id returns 400
    When I send a POST request to /prompts with:
      | content | "no session" |
    Then the response status is 400

  Scenario: Get recent prompts
    When I send a GET request to /prompts/recent
    Then the response status is 200
    And the response body is an array
    And items are ordered by created_at descending

  Scenario: Get recent prompts with limit
    When I send a GET request to /prompts/recent?limit=5
    Then the response array has at most 5 items

  Scenario: Search prompts by content
    Given there are prompts with varied content
    When I send a GET request to /prompts/search?q=architecture
    Then the response status is 200
    And the response body is an array
    And results contain "architecture" in their content

  Scenario: Search prompts with project filter
    When I send a GET request to /prompts/search?q=architecture&project=myapp
    Then all results have project "myapp"

  Scenario: Search prompts without query returns 400
    When I send a GET request to /prompts/search
    Then the response status is 400

  Scenario: Delete a prompt
    Given a prompt with id "1" exists
    When I send a DELETE request to /prompts/1
    Then the response status is 204

  Scenario: Delete non-existent prompt returns 404
    When I send a DELETE request to /prompts/9999
    Then the response status is 404
```

## Análisis breve

- **Qué pide realmente:** 4 endpoints: POST /prompts (guardar), GET /prompts/recent (listar), GET /prompts/search (FTS5 search), DELETE /prompts/{id}. Reusa FTS5 de la tabla prompts_fts.
- **Módulos sospechados:** `internal/api/prompts.go` (handlers), `internal/store/prompt.go` (repo puede existir de HU-01.6), FTS5 via prompts_fts
- **Riesgos / dependencias:** Depende de HU-01.6 (prompt storage) para store layer y de HU-01.1 para FTS5 en prompts_fts
- **Esfuerzo tentativo:** S

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
